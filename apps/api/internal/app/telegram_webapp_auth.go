package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// telegramWebappAuthExpiry 是 initData auth_date 允许的最大有效期。
const telegramWebappAuthExpiry = 24 * time.Hour

// handleTelegramWebappAuth 校验 Telegram Mini App 的 initData，并签发会话。
// 请求体：{ "initData": "query_id=...&user=...&hash=..." }
func (a *App) handleTelegramWebappAuth(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{"error": "telegram bot not configured"})
		return
	}
	var req struct {
		InitData string `json:"initData"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid body"})
		return
	}
	if strings.TrimSpace(req.InitData) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "missing initData"})
		return
	}
	fields, err := a.telegramValidateInitData(req.InitData, a.cfg.TelegramBotToken)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": err.Error()})
		return
	}
	tgUserID, err := telegramInitDataUserID(fields["user"])
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "invalid user field"})
		return
	}
	var userID string
	err = a.db.QueryRowContext(r.Context(), `SELECT user_id FROM telegram_bindings WHERE chat_id=?`, tgUserID).Scan(&userID)
	if err != nil {
		respondJSON(w, http.StatusForbidden, map[string]interface{}{"error": "未绑定账号，请先在 Telegram 中使用 /bind 完成绑定。"})
		return
	}
	if err := a.issueSession(w, r, userID); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// telegramValidateInitData 校验 Telegram initData 的 HMAC 签名与有效期。
// 返回解析后的字段（不含 hash）。
func (a *App) telegramValidateInitData(initData, botToken string) (map[string]string, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, errors.New("invalid initData")
	}
	hash := values.Get("hash")
	values.Del("hash")
	if hash == "" {
		return nil, errors.New("missing hash")
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var dataCheck strings.Builder
	for i, key := range keys {
		if i > 0 {
			dataCheck.WriteString("\n")
		}
		dataCheck.WriteString(key)
		dataCheck.WriteString("=")
		dataCheck.WriteString(values.Get(key))
	}
	secretKey := telegramHMACSHA256([]byte("WebAppData"), []byte(botToken))
	computed := telegramHMACSHA256(secretKey, []byte(dataCheck.String()))
	if !hmac.Equal(computed, mustHexDecode(hash)) {
		return nil, errors.New("invalid signature")
	}
	if authDateStr := values.Get("auth_date"); authDateStr != "" {
		if authDate, err := strconv.ParseInt(authDateStr, 10, 64); err == nil {
			authTime := time.Unix(authDate, 0)
			if authTime.Add(telegramWebappAuthExpiry).Before(time.Now()) {
				return nil, errors.New("initData expired")
			}
		}
	}
	fields := make(map[string]string, len(values))
	for key, vals := range values {
		if len(vals) > 0 {
			fields[key] = vals[0]
		}
	}
	return fields, nil
}

// telegramInitDataUserID 从 initData 的 user JSON 字段中解析 Telegram 用户 ID。
func telegramInitDataUserID(userJSON string) (int64, error) {
	if strings.TrimSpace(userJSON) == "" {
		return 0, errors.New("missing user")
	}
	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(userJSON), &u); err != nil {
		return 0, err
	}
	if u.ID == 0 {
		return 0, errors.New("empty user id")
	}
	return u.ID, nil
}

func telegramHMACSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func mustHexDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}
