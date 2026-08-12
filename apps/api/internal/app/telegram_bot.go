package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	telegramBaseURL            = "https://api.telegram.org/bot"
	telegramNotifyPollInterval = 5 * time.Second
	telegramAlertPollInterval  = 60 * time.Second
)

// telegramHTMLEscape 转义 Telegram HTML parse_mode 中需要转义的字符，
// 防止邮件内容中的 <>&" 被当作 HTML 标签解析（注入/格式破坏）。
func telegramHTMLEscape(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 32)
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// telegramWorker 是 Telegram Bot 长轮询主循环，仅在配置了 token 时启动。
// 失败时记录日志并继续重试。offset 在成功处理更新后推进。
func (a *App) telegramBotWorker(ctx context.Context) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return
	}
	a.log.Info("telegram bot worker started")
	offset := int64(0)
	for {
		select {
		case <-ctx.Done():
			a.log.Info("telegram bot worker stopped")
			return
		default:
		}
		updates, err := a.telegramGetUpdates(ctx, offset)
		if err != nil {
			a.log.Warn("telegram getUpdates failed", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		maxUpdateID := offset
		for _, update := range updates {
			if update.UpdateID <= offset {
				continue
			}
			maxUpdateID = update.UpdateID
			if err := a.telegramHandleUpdate(ctx, &update); err != nil {
				a.log.Warn("telegram handle update failed", "update_id", update.UpdateID, "error", err)
			}
		}
		offset = maxUpdateID
		a.telegramComposeCleanup(2 * time.Hour)
	}
}

// telegramGetUpdates 调用 Telegram getUpdates 长轮询接口，返回未处理更新。
func (a *App) telegramGetUpdates(ctx context.Context, offset int64) ([]telegramUpdate, error) {
	endpoint := telegramBaseURL + a.cfg.TelegramBotToken + "/getUpdates"
	values := url.Values{
		"offset":  {strconv.FormatInt(offset, 10)},
		"timeout": {"30"},
		"limit":   {"100"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: time.Duration(a.cfg.TelegramBotTimeoutSeconds+15) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr telegramAPIError
		_ = json.Unmarshal(body, &apiErr)
		return nil, fmt.Errorf("telegram getUpdates status %d: %s", resp.StatusCode, apiErr.Description)
	}
	var result struct {
		Ok      bool             `json:"ok"`
		Result  []telegramUpdate `json:"result"`
		Offset  int64            `json:"offset"`
		Error   *telegramAPIError `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if !result.Ok {
		return nil, fmt.Errorf("telegram getUpdates returned ok=false: %s", result.Error.Description)
	}
	return result.Result, nil
}

// telegramHandleUpdate 分发 update 到合适的处理器。
func (a *App) telegramHandleUpdate(ctx context.Context, update *telegramUpdate) error {
	if update.Message != nil && update.Message.From != nil && update.Message.Chat != nil {
		return a.telegramHandleMessage(ctx, update.Message)
	}
	if update.CallbackQuery != nil && update.CallbackQuery.From != nil {
		return a.telegramHandleCallbackQuery(ctx, update.CallbackQuery)
	}
	return nil
}

// telegramHandleMessage 处理消息（命令路由）。
func (a *App) telegramHandleMessage(ctx context.Context, msg *telegramMsg) error {
	userID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}
	if !strings.HasPrefix(text, "/") {
		return a.telegramHandleComposeInput(ctx, chatID, text)
	}
	return a.telegramDispatchCommand(ctx, userID, chatID, text)
}

// telegramHandleCallbackQuery 处理 inline keyboard 回调。
func (a *App) telegramHandleCallbackQuery(ctx context.Context, cb *telegramCallbackQuery) error {
	chatID := int64(0)
	if cb.Message != nil && cb.Message.Chat != nil {
		chatID = cb.Message.Chat.ID
	}
	userID := int64(0)
	if cb.From != nil {
		userID = cb.From.ID
	}
	return a.telegramDispatchCallback(ctx, userID, chatID, cb.Data, cb.ID)
}

// telegramDispatchCommand 分发文本命令到对应的处理器。
func (a *App) telegramDispatchCommand(ctx context.Context, userID, chatID int64, text string) error {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return a.telegramSendMessage(ctx, chatID, "未知命令")
	}
	cmd := parts[0]
	args := parts[1:]
	switch cmd {
	case "/start":
		return a.handleStart(ctx, userID, chatID, args)
	case "/bind":
		return a.handleBind(ctx, userID, chatID, args)
	case "/unbind":
		return a.handleUnbind(ctx, userID, chatID)
	case "/status":
		return a.handleStatus(ctx, userID, chatID, args)
	case "/inbox":
		return a.handleInbox(ctx, userID, chatID, args)
	case "/open":
		return a.handleOpen(ctx, userID, chatID, args)
	case "/send":
		return a.handleSend(ctx, userID, chatID, args)
	case "/reply":
		return a.handleReply(ctx, userID, chatID, args)
	case "/read":
		return a.handleRead(ctx, userID, chatID, args)
	case "/admin":
		return a.handleAdmin(ctx, userID, chatID, args)
	default:
		return a.telegramSendMessage(ctx, chatID, "未知命令: "+cmd)
	}
}

// telegramDispatchCallback 分发 callback query。
func (a *App) telegramDispatchCallback(ctx context.Context, userID, chatID int64, data, callbackQueryID string) error {
	defer a.telegramAnswerCallbackQuery(callbackQueryID, "")
	parts := strings.SplitN(data, ":", 2)
	action := parts[0]
	payload := ""
	if len(parts) == 2 {
		payload = parts[1]
	}
	switch action {
	case "read":
		if payload != "" {
			return a.handleRead(ctx, userID, chatID, []string{payload})
		}
	case "reply":
		if payload != "" {
			return a.handleReply(ctx, userID, chatID, []string{payload})
		}
	case "open":
		return a.handleOpen(ctx, userID, chatID, []string{payload})
	case "compose":
		return a.handleComposeCallback(ctx, chatID, payload)
	default:
		return a.telegramSendMessage(ctx, chatID, "收到回调: "+data)
	}
	return nil
}

// telegramSendMessage 发送文本消息。
func (a *App) telegramSendMessage(ctx context.Context, chatID int64, text string) error {
	return a.telegramSendMessageKeyboard(ctx, chatID, text, nil)
}

// telegramSendMessageKeyboard 发送文本消息（可选 inline keyboard）。
func (a *App) telegramSendMessageKeyboard(ctx context.Context, chatID int64, text string, keyboard *TelegramInlineKeyboard) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	if keyboard != nil && len(keyboard.InlineKeyboard) > 0 {
		payload["reply_markup"] = keyboard
	}
	_, err := a.telegramCall(ctx, "sendMessage", payload)
	return err
}

// telegramSendPhoto 发送图片。
func (a *App) telegramSendPhoto(ctx context.Context, chatID int64, photoURL string, caption string) error {
	payload := map[string]interface{}{
		"chat_id":  chatID,
		"photo":    photoURL,
		"caption":  caption,
	}
	_, err := a.telegramCall(ctx, "sendPhoto", payload)
	return err
}

// telegramEditMessageText 编辑已发送的文本消息。
func (a *App) telegramEditMessageText(ctx context.Context, chatID, messageID int64, text string) error {
	payload := map[string]interface{}{
		"chat_id":  chatID,
		"message_id": messageID,
		"text":     text,
		"parse_mode": "HTML",
	}
	_, err := a.telegramCall(ctx, "editMessageText", payload)
	return err
}

// telegramAnswerCallbackQuery 回应回调查询。
func (a *App) telegramAnswerCallbackQuery(callbackQueryID, text string) error {
	if strings.TrimSpace(callbackQueryID) == "" {
		return nil
	}
	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
		"text":              text,
	}
	_, err := a.telegramCall(context.Background(), "answerCallbackQuery", payload)
	return err
}

// telegramCall 调用 Telegram Bot API 方法。
func (a *App) telegramCall(ctx context.Context, method string, payload map[string]interface{}) ([]byte, error) {
	endpoint := telegramBaseURL + a.cfg.TelegramBotToken + "/" + method
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(a.cfg.TelegramBotTimeoutSeconds+5) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var apiErr telegramAPIError
		_ = json.Unmarshal(respBody, &apiErr)
		return respBody, fmt.Errorf("telegram %s status %d: %s", method, resp.StatusCode, apiErr.Description)
	}
	return respBody, nil
}

// telegramNotifyWorker 周期性扫描 telegram_notify_outbox 并投递通知。
func (a *App) telegramNotifyWorker(ctx context.Context) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return
	}
	a.log.Info("telegram notify worker started")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.log.Info("telegram notify worker stopped")
			return
		case <-ticker.C:
			if err := a.processDueTelegramNotify(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.log.Warn("telegram notify processing failed", "error", err)
			}
		}
	}
}

// telegramAlertWorker 周期性扫描 telegram_alert_outbox 并投递告警。
func (a *App) telegramAlertWorker(ctx context.Context) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return
	}
	a.log.Info("telegram alert worker started")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.log.Info("telegram alert worker stopped")
			return
		case <-ticker.C:
			a.checkTelegramAlerts(ctx)
			if err := a.processDueTelegramAlerts(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.log.Warn("telegram alert processing failed", "error", err)
			}
		}
	}
}

// telegramUpdate 表示一个 Telegram Bot getUpdates 返回的更新。
type telegramUpdate struct {
	UpdateID     int64            `json:"update_id"`
	Message      *telegramMsg    `json:"message,omitempty"`
	CallbackQuery *telegramCallbackQuery `json:"callback_query,omitempty"`
}

// telegramMsg 表示一个 Telegram 消息。
type telegramMsg struct {
	MessageID int64           `json:"message_id"`
	From      *telegramUser   `json:"from"`
	Chat      *telegramChat   `json:"chat"`
	Text      string          `json:"text,omitempty"`
	Date      int64           `json:"date"`
	ReplyTo   *telegramMsg    `json:"reply_to_message,omitempty"`
}

// telegramUser 表示一个 Telegram 用户。
type telegramUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// telegramChat 表示一个 Telegram 聊天。
type telegramChat struct {
	ID        int64  `json:"id"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Type      string `json:"type,omitempty"`
}

// telegramCallbackQuery 表示一个 Telegram callback query。
type telegramCallbackQuery struct {
	ID      string           `json:"id"`
	From    *telegramUser    `json:"from"`
	Message *telegramMsg     `json:"message,omitempty"`
	Chat    *telegramChat    `json:"chat,omitempty"`
	Data    string           `json:"data,omitempty"`
}

// telegramAPIError 是 Telegram API 返回的失败响应描述。
type telegramAPIError struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// TelegramMessage 是 Telegram Bot 消息结构。
type TelegramMessage struct {
	ChatID      int64
	Text        string
	ParseMode   string
	ReplyMarkup *TelegramInlineKeyboard
}

// TelegramInlineKeyboard 是 Telegram inline keyboard 结构。
type TelegramInlineKeyboard struct {
	InlineKeyboard [][]TelegramInlineKeyboardButton
}

// TelegramInlineKeyboardButton 是 Telegram inline button 结构。
type TelegramInlineKeyboardButton struct {
	Text         string
	URL          string
	CallbackData string
}

// TelegramBinding 表示 Telegram 绑定信息。
type TelegramBinding struct {
	ID            int64
	Code          string
	UserID        int64
	IsAdminTarget int
	CreatedAt     string
}

// getAuthUserID 从请求中获取已认证用户的 ID。
func getAuthUserID(r *http.Request) (int64, error) {
	user := currentUser(r)
	if user == nil {
		return 0, errors.New("未认证")
	}
	if user.ID == "" {
		return 0, errors.New("用户 ID 为空")
	}
	id, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// generateCode 生成随机 6 字符的绑定码。
func generateCode() string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[randInt(0, len(letters)-1)]
	}
	return string(b)
}

// randInt 在 [min, max] 范围内返回随机整数。
func randInt(min, max int) int {
	b := make([]byte, 4)
	_, _ = io.ReadFull(rand.Reader, b)
	v := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	return min + (v%(max-min+1))
}

// handleTelegramWebhook 处理 Telegram webhook 传入的更新。
func (a *App) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	update, ok := body["update"].(map[string]interface{})
	if !ok {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "无效的 Telegram webhook 消息"})
		return
	}
	results, err := a.telegramGetUpdates(r.Context(), int64(update["update_id"].(float64)))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	if len(results) == 0 {
		return
	}
	for _, update := range results {
		if update.Message != nil {
			a.telegramDispatchCommand(r.Context(), update.Message.From.ID, update.Message.Chat.ID, update.Message.Text)
		} else if update.CallbackQuery != nil {
			userID := int64(0)
			if update.CallbackQuery.From != nil {
				userID = update.CallbackQuery.From.ID
			}
			chatID := int64(0)
			if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
				chatID = update.CallbackQuery.Message.Chat.ID
			}
			a.telegramDispatchCallback(r.Context(), userID, chatID, update.CallbackQuery.Data, update.CallbackQuery.ID)
		}
	}
}

// generateTelegramBindingCode 生成绑定码并存储到 Telegram 绑定表中。
func (a *App) generateTelegramBindingCode(ctx context.Context, userID int64) (string, error) {
	code := generateCode()
	now := a.now().UTC().Format(time.RFC3339Nano)
	_, err := a.db.ExecContext(ctx, `INSERT INTO telegram_binding_codes(code, user_id, created_at) VALUES(?,?,?)`, code, userID, now)
	return code, err
}

// handleGenerateTelegramBindingCode 生成 Telegram 绑定码。
func (a *App) handleGenerateTelegramBindingCode(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	code, err := a.generateTelegramBindingCode(r.Context(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"code": code})
}

// handleListTelegramBindings 列出 Telegram 绑定。
func (a *App) handleListTelegramBindings(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	rows, err := a.db.QueryContext(r.Context(),
		`SELECT id, code, user_id, is_admin_target, created_at FROM telegram_bindings WHERE user_id=?`, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	defer rows.Close()
	var bindings []TelegramBinding
	for rows.Next() {
		var b TelegramBinding
		if err := rows.Scan(&b.ID, &b.Code, &b.UserID, &b.IsAdminTarget, &b.CreatedAt); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}
		bindings = append(bindings, b)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"bindings": bindings})
}

// handleDeleteTelegramBinding 删除 Telegram 绑定。
func (a *App) handleDeleteTelegramBinding(w http.ResponseWriter, r *http.Request) {
	userID, err := getAuthUserID(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	bindingID := chi.URLParam(r, "id")
	_, err = a.db.ExecContext(r.Context(), `DELETE FROM telegram_bindings WHERE id=? AND user_id=?`, bindingID, userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
