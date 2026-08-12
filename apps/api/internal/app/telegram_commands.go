package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// TelegramNotifyMeta 是邮件通知入队的元数据。
type TelegramNotifyMeta struct {
	Subject  string
	FromAddr string
	FromName string
	Snippet  string
}

// processDueTelegramNotify 处理 telegram_notify_outbox 中待发送的通知。
func (a *App) processDueTelegramNotify(ctx context.Context) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, chat_id, mailbox_id, user_id, subject, from_addr, from_name, snippet,
		  attempt_count, max_attempts, next_attempt_at, created_at, delivered_at
		 FROM telegram_notify_outbox
		 WHERE delivered_at IS NULL AND next_attempt_at <= ?
		 ORDER BY next_attempt_at, created_at LIMIT 20`,
		a.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, mailboxID, userID, subject, fromAddr, fromName, snippet,
			createdAt, deliveredAt, nextAttemptAt string
		var chatID int64
		var attemptCount, maxAttempts int
		if err := rows.Scan(&id, &chatID, &mailboxID, &userID,
			&subject, &fromAddr, &fromName, &snippet,
			&attemptCount, &maxAttempts, &nextAttemptAt, &createdAt, &deliveredAt); err != nil {
			continue
		}

		if attemptCount >= maxAttempts {
			_, _ = a.db.ExecContext(ctx, `DELETE FROM telegram_notify_outbox WHERE id=?`, id)
			continue
		}

		text := fmt.Sprintf("📬 邮件通知\n\nSubject: %s\nFrom: %s\n%s", subject, fromName, snippet)
		if err := a.telegramSendMessage(ctx, chatID, text); err != nil {
			_, _ = a.db.ExecContext(ctx,
				`UPDATE telegram_notify_outbox SET attempt_count=attempt_count+1, next_attempt_at=? WHERE id=?`,
				a.now().UTC().Add(30*time.Second).Format(time.RFC3339Nano), id)
			continue
		}

		_, _ = a.db.ExecContext(ctx,
			`UPDATE telegram_notify_outbox SET delivered_at=? WHERE id=?`,
			a.now().UTC().Format(time.RFC3339Nano), id)
	}
	return nil
}

// processDueTelegramAlerts 处理 telegram_alert_outbox 中待发送的告警。
func (a *App) processDueTelegramAlerts(ctx context.Context) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, alert_type, chat_id, title, body, dedupe_key, attempt_count, max_attempts, next_attempt_at, created_at, delivered_at
		 FROM telegram_alert_outbox
		 WHERE delivered_at IS NULL AND next_attempt_at <= ?
		 ORDER BY next_attempt_at, created_at LIMIT 20`,
		a.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, alertType, title, body, dedupeKey, createdAt, deliveredAt, nextAttemptAt string
		var chatID int64
		var attemptCount, maxAttempts int
		if err := rows.Scan(&id, &alertType, &chatID, &title, &body,
			&dedupeKey, &attemptCount, &maxAttempts, &nextAttemptAt, &createdAt, &deliveredAt); err != nil {
			continue
		}

		if attemptCount >= maxAttempts {
			_, _ = a.db.ExecContext(ctx, `DELETE FROM telegram_alert_outbox WHERE id=?`, id)
			continue
		}

		var text string
		switch alertType {
		case "smtp_connect":
			text = fmt.Sprintf("⚠️ SMTP 连接告警\n\n次数: %d\n\n%s", attemptCount, body)
		case "db_error":
			text = fmt.Sprintf("⚠️ 数据库错误告警\n\n次数: %d\n\n%s", attemptCount, body)
		default:
			text = fmt.Sprintf("📢 %s\n\n%s", title, body)
		}

		if err := a.telegramSendMessage(ctx, chatID, text); err != nil {
			_, _ = a.db.ExecContext(ctx,
				`UPDATE telegram_alert_outbox SET attempt_count=attempt_count+1, next_attempt_at=? WHERE id=?`,
				a.now().UTC().Add(30*time.Second).Format(time.RFC3339Nano), id)
			continue
		}

		_, _ = a.db.ExecContext(ctx,
			`UPDATE telegram_alert_outbox SET delivered_at=? WHERE id=?`,
			a.now().UTC().Format(time.RFC3339Nano), id)
	}
	return nil
}

// enqueueTelegramNotify 将新邮件通知入队到 telegram_notify_outbox。
func (a *App) enqueueTelegramNotify(ctx context.Context, mailboxID, userID, folderName, messageID string, meta TelegramNotifyMeta) error {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return nil
	}
	if !a.cfg.TelegramNotifyEnabled {
		return nil
	}
	// Spam 过滤：若为 Spam 文件夹且 notify_spam 为 0 则跳过
	if strings.EqualFold(folderName, "Spam") {
		var notifySpam int
		_ = a.db.QueryRowContext(ctx, `SELECT notify_spam FROM telegram_mailbox_settings WHERE mailbox_id=?`, mailboxID).Scan(&notifySpam)
		if notifySpam == 0 {
			return nil
		}
	}
	// 邮箱通知开关
	var notifyEnabled int
	err := a.db.QueryRowContext(ctx, `SELECT COALESCE(notify_enabled, 1) FROM telegram_mailbox_settings WHERE mailbox_id=?`, mailboxID).Scan(&notifyEnabled)
	if err != nil && err != sql.ErrNoRows {
		return nil
	}
	// 查 chat_id
	var chatID int64
	err = a.db.QueryRowContext(ctx, `SELECT chat_id FROM telegram_bindings WHERE user_id=?`, userID).Scan(&chatID)
	if err != nil {
		return nil
	}
	dedupeKey := mailboxID + ":" + messageID
	id := newID("tn")
	now := a.now().UTC().Format(time.RFC3339Nano)
	query := insertIgnoreSQL(a.cfg.DBDriver,
		`INSERT INTO telegram_notify_outbox(id, mailbox_id, user_id, message_id, chat_id,
		   subject, from_addr, from_name, snippet, dedupe_key,
		   attempt_count, max_attempts, next_attempt_at, delivered_at, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		`(dedupe_key)`)
	_, err = a.db.ExecContext(ctx, query,
		id, mailboxID, userID, messageID, chatID,
		meta.Subject, meta.FromAddr, meta.FromName, meta.Snippet, dedupeKey,
		0, 5, now, "", now)
	return err
}

// handleStart 响应 /start 命令。
func (a *App) handleStart(ctx context.Context, userID, chatID int64, args []string) error {
	help := "欢迎使用 EOOS Email Bot。\n\n" +
		"可用命令：\n" +
		"/bind <code> - 使用 Web 设置页生成的绑定码完成绑定\n" +
		"/unbind - 解除当前绑定\n" +
		"/status - 查看绑定状态\n" +
		"/inbox - 查看最近邮件\n" +
		"/read <messageId> - 查看邮件详情\n" +
		"/send - 引导发信\n" +
		"/reply <messageId> - 引导回复\n" +
		"/open - 打开 Mini App\n" +
		"/admin users|domains|mailboxes|disable <email>|enable <email> - 管理命令（仅管理员）"
	return a.telegramSendMessage(ctx, chatID, help)
}

// handleBind 响应 /bind <code> 命令，校验绑定码并完成绑定。
func (a *App) handleBind(ctx context.Context, userID, chatID int64, args []string) error {
	if len(args) == 0 {
		return a.telegramSendMessage(ctx, chatID, "用法：/bind <绑定码>\n\n请先在 Web 设置页生成绑定码，再使用此命令完成绑定。")
	}
	code := strings.TrimSpace(args[0])

	var existing int
	if err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM telegram_bindings WHERE chat_id=?`, chatID).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return a.telegramSendMessage(ctx, chatID, "此 Telegram 聊天已绑定账号。如需更换，请先用 /unbind 解绑。")
	}

	var (
		storedUserID string
		storedChatID int64
		expiresAt    string
		usedAt        string
	)
	err := a.db.QueryRowContext(ctx,
		`SELECT user_id, chat_id, expires_at, used_at FROM telegram_binding_codes WHERE code=?`,
		code).Scan(&storedUserID, &storedChatID, &expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return a.telegramSendMessage(ctx, chatID, "绑定码无效或不存在。")
	}
	if err != nil {
		return err
	}
	if usedAt != "" {
		return a.telegramSendMessage(ctx, chatID, "绑定码已使用，请重新生成。")
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || expiry.Before(a.now().UTC()) {
		return a.telegramSendMessage(ctx, chatID, "绑定码已过期，请重新生成。")
	}

	now := a.now().UTC().Format(time.RFC3339Nano)
	isAdminTarget := 0
	for _, adminChatIDStr := range strings.Split(a.cfg.TelegramAdminChatIDs, ",") {
		adminChatID, _ := strconv.ParseInt(strings.TrimSpace(adminChatIDStr), 10, 64)
		if adminChatID == chatID {
			isAdminTarget = 1
			break
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE telegram_binding_codes SET used_at=? WHERE code=?`, now, code); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telegram_bindings(chat_id, user_id, is_admin_target, created_at, updated_at) VALUES(?,?,?,?,?)`,
		chatID, storedUserID, isAdminTarget, now, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return a.telegramSendMessage(ctx, chatID, "绑定成功！现在可使用 Bot 命令管理邮箱。")
}

// handleUnbind 响应 /unbind 命令。
func (a *App) handleUnbind(ctx context.Context, userID, chatID int64) error {
	result, err := a.db.ExecContext(ctx, `DELETE FROM telegram_bindings WHERE chat_id=?`, chatID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return a.telegramSendMessage(ctx, chatID, "此 Telegram 聊天尚未绑定账号。")
	}
	_, _ = a.db.ExecContext(ctx, `DELETE FROM telegram_binding_codes WHERE chat_id=?`, chatID)
	return a.telegramSendMessage(ctx, chatID, "已成功解绑。如需重新绑定，请使用 /bind <code>。")
}

// handleStatus 响应 /status 命令。
func (a *App) handleStatus(ctx context.Context, userID, chatID int64, args []string) error {
	var (
		storedUserID string
		isAdminTarget int
		createdAt     string
	)
	err := a.db.QueryRowContext(ctx,
		`SELECT user_id, is_admin_target, created_at FROM telegram_bindings WHERE chat_id=?`,
		chatID).Scan(&storedUserID, &isAdminTarget, &createdAt)
	if err == sql.ErrNoRows {
		return a.telegramSendMessage(ctx, chatID, "此 Telegram 聊天尚未绑定账号。请使用 /bind <code> 完成绑定。")
	}
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("绑定状态：已绑定\n用户 ID: %s\n绑定时间: %s", storedUserID, createdAt)
	if isAdminTarget == 1 {
		msg += "\n角色: 管理员"
	}
	return a.telegramSendMessage(ctx, chatID, msg)
}

// handleInbox 响应 /inbox 命令，返回最近邮件列表。
func (a *App) handleInbox(ctx context.Context, userID, chatID int64, args []string) error {
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT m.id, m.subject, COALESCE(m.from_name,''), m.from_addr, m.received_at
		 FROM messages m
		 JOIN folders f ON f.id=m.folder_id
		 JOIN mailboxes mb ON mb.id=m.mailbox_id
		 WHERE mb.user_id=? AND lower(f.name)='inbox'
		 ORDER BY m.received_at DESC LIMIT 5`, storedUserID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("最近 5 封收件箱邮件：\n\n")
	count := 0
	for rows.Next() {
		var id, subject, fromName, fromAddr, receivedAt string
		if err := rows.Scan(&id, &subject, &fromName, &fromAddr, &receivedAt); err != nil {
			return err
		}
		count++
		b.WriteString(fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", count, subject, fromAddr, receivedAt))
	}
	if count == 0 {
		return a.telegramSendMessage(ctx, chatID, "收件箱暂无邮件。")
	}
	return a.telegramSendMessage(ctx, chatID, b.String())
}

// handleRead 响应 /read <messageId> 命令，返回邮件详情。
func (a *App) handleRead(ctx context.Context, userID, chatID int64, args []string) error {
	if len(args) == 0 {
		return a.telegramSendMessage(ctx, chatID, "用法：/read <messageId>")
	}
	messageID := strings.TrimSpace(args[0])
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	var (
		subject   string
		fromName  string
		fromAddr  string
		snippet   string
		receivedAt string
	)
	err = a.db.QueryRowContext(ctx,
		`SELECT m.subject, COALESCE(m.from_name,''), m.from_addr, COALESCE(m.snippet,''), m.received_at
		 FROM messages m
		 JOIN mailboxes mb ON mb.id=m.mailbox_id
		 WHERE m.id=? AND mb.user_id=?`, messageID, storedUserID).Scan(&subject, &fromName, &fromAddr, &snippet, &receivedAt)
	if err == sql.ErrNoRows {
		return a.telegramSendMessage(ctx, chatID, "邮件不存在或您无权访问。")
	}
	if err != nil {
		return err
	}
	if fromName != "" {
		fromAddr = fromName + " <" + fromAddr + ">"
	}
	text := fmt.Sprintf("发件人: %s\n主题: %s\n时间: %s\n\n摘要:\n%s", fromAddr, subject, receivedAt, snippet)
	return a.telegramSendMessage(ctx, chatID, text)
}

// handleSend 响应 /send 命令，启动发信会话状态机。
func (a *App) handleSend(ctx context.Context, userID, chatID int64, args []string) error {
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	_ = storedUserID
	return a.telegramSendMessage(ctx, chatID, "发信会话尚未实现，请通过 Web 界面发送邮件。")
}

// handleReply 响应 /reply <messageId>。
func (a *App) handleReply(ctx context.Context, userID, chatID int64, args []string) error {
	if len(args) == 0 {
		return a.telegramSendMessage(ctx, chatID, "用法：/reply <messageId>")
	}
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	_ = storedUserID
	return a.telegramSendMessage(ctx, chatID, "回复会话尚未实现，请通过 Web 界面回复邮件。")
}

// handleOpen 响应 /open 命令，返回 Mini App URL。
func (a *App) handleOpen(ctx context.Context, userID, chatID int64, args []string) error {
	publicURL := strings.TrimRight(strings.TrimSpace(a.cfg.PublicBaseURL), "/")
	if publicURL == "" {
		return a.telegramSendMessage(ctx, chatID, "Mini App URL 未配置。请联系管理员设置 PublicBaseURL。")
	}
	webAppURL := publicURL + "/telegram"
	if len(args) > 0 {
		messageID := strings.TrimSpace(args[0])
		if messageID != "" {
			webAppURL += "?mail=" + url.QueryEscape(messageID)
		}
	}
	text := fmt.Sprintf("点击以下链接打开 Mini App (在 Telegram 内置浏览器中打开):\n%s", webAppURL)
	return a.telegramSendMessage(ctx, chatID, text)
}

// handleAdmin 响应 /admin 子命令，仅对绑定账号为 admin 角色的用户可用。
func (a *App) handleAdmin(ctx context.Context, userID, chatID int64, args []string) error {
	if len(args) == 0 {
		return a.telegramSendMessage(ctx, chatID, "用法：/admin users|domains|mailboxes|disable <email>|enable <email>")
	}
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	var role string
	err = a.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id=?`, storedUserID).Scan(&role)
	if err != nil {
		return err
	}
	if role != "admin" {
		return a.telegramSendMessage(ctx, chatID, "无权限：仅管理员可执行管理命令。")
	}
	subCmd := args[0]
	switch subCmd {
	case "users":
		return a.tgAdminListUsers(ctx, chatID)
	case "domains":
		return a.tgAdminListDomains(ctx, chatID)
	case "mailboxes":
		return a.tgAdminListMailboxes(ctx, chatID)
	case "disable":
		if len(args) < 2 {
			return a.telegramSendMessage(ctx, chatID, "用法：/admin disable <email>")
		}
		return a.tgAdminToggleUser(ctx, chatID, args[1], true)
	case "enable":
		if len(args) < 2 {
			return a.telegramSendMessage(ctx, chatID, "用法：/admin enable <email>")
		}
		return a.tgAdminToggleUser(ctx, chatID, args[1], false)
	default:
		return a.telegramSendMessage(ctx, chatID, "未知子命令: "+subCmd)
	}
}

// tgAdminListUsers 响应 /admin users 命令。
func (a *App) tgAdminListUsers(ctx context.Context, chatID int64) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, email, role, disabled, created_at FROM users ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("用户列表（最多 20 条）：\n\n")
	count := 0
	for rows.Next() {
		var id, email, role, createdAt string
		var disabled int
		if err := rows.Scan(&id, &email, &role, &disabled, &createdAt); err != nil {
			return err
		}
		count++
		status := "active"
		if disabled == 1 {
			status = "disabled"
		}
		b.WriteString(fmt.Sprintf("%d. %s [%s/%s]\n", count, email, role, status))
	}
	if count == 0 {
		return a.telegramSendMessage(ctx, chatID, "暂无用户。")
	}
	return a.telegramSendMessage(ctx, chatID, b.String())
}

// tgAdminListDomains 响应 /admin domains 命令。
func (a *App) tgAdminListDomains(ctx context.Context, chatID int64) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, name, status FROM domains ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("域名列表（最多 20 条）：\n\n")
	count := 0
	for rows.Next() {
		var id, name, status string
		if err := rows.Scan(&id, &name, &status); err != nil {
			return err
		}
		count++
		b.WriteString(fmt.Sprintf("%d. %s [%s]\n", count, name, status))
	}
	if count == 0 {
		return a.telegramSendMessage(ctx, chatID, "暂无域名。")
	}
	return a.telegramSendMessage(ctx, chatID, b.String())
}

// tgAdminListMailboxes 响应 /admin mailboxes 命令。
func (a *App) tgAdminListMailboxes(ctx context.Context, chatID int64) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT mb.id, mb.address, mb.status, u.email FROM mailboxes mb JOIN users u ON u.id=mb.user_id ORDER BY mb.created_at DESC LIMIT 20`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("邮箱列表（最多 20 条）：\n\n")
	count := 0
	for rows.Next() {
		var id, addr, status, owner string
		if err := rows.Scan(&id, &addr, &status, &owner); err != nil {
			return err
		}
		count++
		b.WriteString(fmt.Sprintf("%d. %s [%s] owner=%s\n", count, addr, status, owner))
	}
	if count == 0 {
		return a.telegramSendMessage(ctx, chatID, "暂无邮箱。")
	}
	return a.telegramSendMessage(ctx, chatID, b.String())
}

// tgAdminToggleUser 响应 /admin disable/enable 命令。
func (a *App) tgAdminToggleUser(ctx context.Context, chatID int64, email string, disable bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	var q string
	if disable {
		q = `UPDATE users SET disabled=1 WHERE email=?`
	} else {
		q = `UPDATE users SET disabled=0 WHERE email=?`
	}
	result, err := a.db.ExecContext(ctx, q, email)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("未找到邮箱 %s 的用户。", email))
	}
	action := "启用"
	if disable {
		action = "禁用"
	}
	return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("已%s用户 %s。", action, email))
}

// telegramLookupBoundUser 通过 chatID 查找已绑定的 user_id。
func (a *App) telegramLookupBoundUser(ctx context.Context, chatID int64) (string, error) {
	var userID string
	err := a.db.QueryRowContext(ctx,
		`SELECT user_id FROM telegram_bindings WHERE chat_id=?`, chatID).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", errors.New("此聊天尚未绑定账号，请先使用 /bind <code> 完成绑定")
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}