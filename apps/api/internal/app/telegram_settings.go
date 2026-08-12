package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// telegramSettingsResponse 是 GET /api/me/telegram/settings 的响应。
type telegramSettingsResponse struct {
	Enabled        bool                      `json:"enabled"`
	BotConfigured  bool                      `json:"botConfigured"`
	Bound          bool                      `json:"bound"`
	BindingCode    string                    `json:"bindingCode,omitempty"`
	Mailboxes      []telegramMailboxSettings `json:"mailboxes"`
	BindingChatID  int64                     `json:"bindingChatId,omitempty"`
}

// telegramMailboxSettings 是单个邮箱的 Telegram 通知设置。
type telegramMailboxSettings struct {
	MailboxID     string `json:"mailboxId"`
	Address       string `json:"address"`
	DisplayName   string `json:"displayName"`
	NotifyEnabled bool   `json:"notifyEnabled"`
	NotifySpam    bool   `json:"notifySpam"`
}

// handleGetTelegramSettings 返回当前用户的 Telegram 绑定与通知设置。
func (a *App) handleGetTelegramSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	resp := telegramSettingsResponse{
		Enabled:       a.cfg.TelegramNotifyEnabled,
		BotConfigured: strings.TrimSpace(a.cfg.TelegramBotToken) != "",
	}

	var chatID int64
	err = a.db.QueryRowContext(r.Context(), `SELECT chat_id FROM telegram_bindings WHERE user_id=?`, userID).Scan(&chatID)
	if err == nil {
		resp.Bound = true
		resp.BindingChatID = chatID
	} else if err != nil && err != sql.ErrNoRows {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}

	rows, err := a.db.QueryContext(r.Context(),
		`SELECT mb.id, mb.address, COALESCE(mb.display_name,''), COALESCE(ts.notify_enabled,1), COALESCE(ts.notify_spam,0)
		 FROM mailboxes mb
		 LEFT JOIN telegram_mailbox_settings ts ON ts.mailbox_id=mb.id
		 WHERE mb.user_id=? AND mb.status='active'
		 ORDER BY mb.created_at`, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s telegramMailboxSettings
		var notifyEnabled, notifySpam int
		if err := rows.Scan(&s.MailboxID, &s.Address, &s.DisplayName, &notifyEnabled, &notifySpam); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}
		s.NotifyEnabled = notifyEnabled != 0
		s.NotifySpam = notifySpam != 0
		resp.Mailboxes = append(resp.Mailboxes, s)
	}
	respondJSON(w, http.StatusOK, resp)
}

// handleUpdateTelegramMailboxSettings 更新指定邮箱的 Telegram 通知开关。
func (a *App) handleUpdateTelegramMailboxSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	mailboxID := chi.URLParam(r, "mailboxId")
	if mailboxID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "missing mailboxId"})
		return
	}
	var owned int
	if err := a.db.QueryRowContext(r.Context(), `SELECT 1 FROM mailboxes WHERE id=? AND user_id=?`, mailboxID, userID).Scan(&owned); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{"error": "mailbox not found"})
		return
	}
	var req struct {
		NotifyEnabled *bool `json:"notifyEnabled"`
		NotifySpam    *bool `json:"notifySpam"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid body"})
		return
	}
	if req.NotifyEnabled == nil && req.NotifySpam == nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "no fields to update"})
		return
	}
	now := a.now().UTC().Format(time.RFC3339Nano)
	query := upsertSQL(a.cfg.DBDriver,
		`INSERT INTO telegram_mailbox_settings(mailbox_id, notify_enabled, notify_spam, updated_at) VALUES(?,?,?,?)`,
		`(mailbox_id)`,
		`notify_enabled=excluded.notify_enabled,notify_spam=excluded.notify_spam,updated_at=excluded.updated_at`,
		`notify_enabled=VALUES(notify_enabled),notify_spam=VALUES(notify_spam),updated_at=VALUES(updated_at)`)
	notifyEnabled, notifySpam := 1, 0
	if req.NotifyEnabled != nil {
		notifyEnabled = boolInt(*req.NotifyEnabled)
	}
	if req.NotifySpam != nil {
		notifySpam = boolInt(*req.NotifySpam)
	}
	_, err = a.db.ExecContext(r.Context(), query, mailboxID, notifyEnabled, notifySpam, now)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
