package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// telegramComposeStep 表示发送会话状态机的当前步骤。
type telegramComposeStep int

const (
	composeStepMailbox telegramComposeStep = iota
	composeStepTo
	composeStepSubject
	composeStepBody
	composeStepConfirm
)

// telegramComposeSession 是发送/回复会话状态机的内存状态。
type telegramComposeSession struct {
	chatID      int64
	userID      string
	mailboxID   string
	mailboxAddr string
	step        telegramComposeStep
	to          []string
	subject     string
	body        string
	replyToMsg  string
	createdAt   time.Time
}

// handleSend 响应 /send 命令，启动发信会话状态机。
func (a *App) handleSend(ctx context.Context, userID, chatID int64, args []string) error {
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	a.telegramComposeMu.Lock()
	a.telegramCompose[chatID] = &telegramComposeSession{chatID: chatID, userID: storedUserID, step: composeStepMailbox, createdAt: a.now()}
	a.telegramComposeMu.Unlock()
	return a.telegramPromptMailbox(ctx, chatID, storedUserID)
}

// handleReply 响应 /reply <messageId> 命令，启动回复会话状态机。
func (a *App) handleReply(ctx context.Context, userID, chatID int64, args []string) error {
	if len(args) == 0 {
		return a.telegramSendMessage(ctx, chatID, "用法：/reply <messageId>")
	}
	messageID := strings.TrimSpace(args[0])
	storedUserID, err := a.telegramLookupBoundUser(ctx, chatID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, err.Error())
	}
	var (
		fromAddr string
		subject  string
	)
	err = a.db.QueryRowContext(ctx,
		`SELECT m.from_addr, m.subject FROM messages m JOIN mailboxes mb ON mb.id=m.mailbox_id WHERE m.id=? AND mb.user_id=?`,
		messageID, storedUserID).Scan(&fromAddr, &subject)
	if err == sql.ErrNoRows {
		return a.telegramSendMessage(ctx, chatID, "邮件不存在或您无权访问。")
	}
	if err != nil {
		return err
	}
	replySubject := "Re: " + subject
	a.telegramComposeMu.Lock()
	a.telegramCompose[chatID] = &telegramComposeSession{
		chatID:     chatID,
		userID:     storedUserID,
		step:       composeStepMailbox,
		to:         []string{fromAddr},
		subject:    replySubject,
		replyToMsg: messageID,
		createdAt:  a.now(),
	}
	a.telegramComposeMu.Unlock()
	return a.telegramPromptMailbox(ctx, chatID, storedUserID)
}

// telegramPromptMailbox 提示用户选择发送邮箱。
func (a *App) telegramPromptMailbox(ctx context.Context, chatID int64, userID string) error {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, address FROM mailboxes WHERE user_id=? AND status='active' ORDER BY created_at`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type mailboxOption struct {
		id      string
		address string
	}
	var options []mailboxOption
	for rows.Next() {
		var o mailboxOption
		if err := rows.Scan(&o.id, &o.address); err != nil {
			return err
		}
		options = append(options, o)
	}
	if len(options) == 0 {
		return a.telegramSendMessage(ctx, chatID, "您名下没有可用的发送邮箱，请先在 Web 界面创建邮箱。")
	}
	if len(options) == 1 {
		a.telegramComposeMu.Lock()
		sess := a.telegramCompose[chatID]
		if sess != nil {
			sess.mailboxID = options[0].id
			sess.mailboxAddr = options[0].address
			sess.step = composeStepTo
		}
		a.telegramComposeMu.Unlock()
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("发送邮箱: %s\n请输入收件人邮箱（多个用逗号分隔）:", telegramHTMLEscape(options[0].address)))
	}
	kb := &TelegramInlineKeyboard{}
	for i := range options {
		kb.InlineKeyboard = append(kb.InlineKeyboard, []TelegramInlineKeyboardButton{{
			Text: options[i].address, CallbackData: "compose:mailbox:" + options[i].id,
		}})
	}
	return a.telegramSendMessageKeyboard(ctx, chatID, "请选择发送邮箱:", kb)
}

// handleComposeCallback 处理发送会话的 inline keyboard 回调。
func (a *App) handleComposeCallback(ctx context.Context, chatID int64, payload string) error {
	if payload == "" {
		return nil
	}
	a.telegramComposeMu.Lock()
	defer a.telegramComposeMu.Unlock()
	sess := a.telegramCompose[chatID]
	if sess == nil {
		return a.telegramSendMessage(ctx, chatID, "当前没有进行中的发信会话。请使用 /send 或 /reply 开始。")
	}
	parts := strings.SplitN(payload, ":", 2)
	action := parts[0]
	arg := ""
	if len(parts) == 2 {
		arg = parts[1]
	}
	switch action {
	case "mailbox":
		if arg == "" {
			return nil
		}
		var address string
		err := a.db.QueryRowContext(ctx, `SELECT address FROM mailboxes WHERE id=? AND user_id=? AND status='active'`, arg, sess.userID).Scan(&address)
		if err != nil {
			return a.telegramSendMessage(ctx, chatID, "邮箱不存在或不可用。")
		}
		sess.mailboxID = arg
		sess.mailboxAddr = address
		sess.step = composeStepTo
		if sess.replyToMsg != "" {
			sess.step = composeStepSubject
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("收件人: %s\n主题: %s\n请输入邮件正文:", telegramHTMLEscape(strings.Join(sess.to, ", ")), telegramHTMLEscape(sess.subject)))
	}
	return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("发送邮箱: %s\n请输入收件人邮箱（多个用逗号分隔）:", telegramHTMLEscape(address)))
	case "confirm":
		if arg == "no" {
			delete(a.telegramCompose, chatID)
			return a.telegramSendMessage(ctx, chatID, "已取消发送。")
		}
		if arg != "yes" {
			return nil
		}
		return a.telegramComposeSend(ctx, chatID, sess)
	default:
		return a.telegramSendMessage(ctx, chatID, "未知操作。")
	}
}

// telegramHandleComposeInput 处理发送会话中的非命令文本输入。
func (a *App) telegramHandleComposeInput(ctx context.Context, chatID int64, text string) error {
	a.telegramComposeMu.Lock()
	defer a.telegramComposeMu.Unlock()
	sess := a.telegramCompose[chatID]
	if sess == nil {
		return a.telegramSendMessage(ctx, chatID, "当前没有进行中的发信会话。请使用 /send 或 /reply 开始。")
	}
	if strings.EqualFold(strings.TrimSpace(text), "取消") || strings.EqualFold(strings.TrimSpace(text), "/cancel") {
		delete(a.telegramCompose, chatID)
		return a.telegramSendMessage(ctx, chatID, "已取消发送。")
	}
	switch sess.step {
	case composeStepTo:
		recipients := splitRecipients(text)
		if len(recipients) == 0 {
			return a.telegramSendMessage(ctx, chatID, "请输入至少一个收件人邮箱。")
		}
		sess.to = recipients
		sess.step = composeStepSubject
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("收件人: %s\n请输入邮件主题:", strings.Join(recipients, ", ")))
	case composeStepSubject:
		sess.subject = strings.TrimSpace(text)
		if sess.subject == "" {
			sess.subject = "(no subject)"
		}
		sess.step = composeStepBody
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("主题: %s\n请输入邮件正文:", telegramHTMLEscape(sess.subject)))
	case composeStepBody:
		sess.body = strings.TrimSpace(text)
		sess.step = composeStepConfirm
		kb := &TelegramInlineKeyboard{
			InlineKeyboard: [][]TelegramInlineKeyboardButton{
				{
					{Text: "确认发送", CallbackData: "compose:confirm:yes"},
					{Text: "取消", CallbackData: "compose:confirm:no"},
				},
			},
		}
		preview := fmt.Sprintf("发送邮箱: %s\n收件人: %s\n主题: %s\n正文:\n%s\n\n确认发送？", telegramHTMLEscape(sess.mailboxAddr), strings.Join(sess.to, ", "), telegramHTMLEscape(sess.subject), telegramHTMLEscape(sess.body))
		return a.telegramSendMessageKeyboard(ctx, chatID, preview, kb)
	default:
		return a.telegramSendMessage(ctx, chatID, "当前会话状态无法处理该输入，请取消后重新开始。")
	}
}

// telegramComposeSend 执行发送并清理会话。
func (a *App) telegramComposeSend(ctx context.Context, chatID int64, sess *telegramComposeSession) error {
	user, err := a.userByID(ctx, sess.userID)
	if err != nil {
		return a.telegramSendMessage(ctx, chatID, "用户不存在或已被停用。")
	}
	mb, err := a.mailboxForUserByID(ctx, sess.userID, sess.mailboxID)
	if err != nil {
		delete(a.telegramCompose, chatID)
		return a.telegramSendMessage(ctx, chatID, "发送邮箱不可用。")
	}
	req := mailComposeInput{
		MailboxID: mb.ID,
		To:        sess.to,
		Subject:   sess.subject,
		Text:      sess.body,
	}
	msg, err := a.sendMailWithSource(ctx, user, mb, req, sendSourceTelegram)
	if err != nil {
		delete(a.telegramCompose, chatID)
		return a.telegramSendMessage(ctx, chatID, "发送失败: "+err.Error())
	}
	delete(a.telegramCompose, chatID)
	if msg != nil {
		return a.telegramSendMessage(ctx, chatID, fmt.Sprintf("邮件已发送。\nMessageID: %s\n主题: %s\n收件人: %s", msg.ID, telegramHTMLEscape(sess.subject), strings.Join(sess.to, ", ")))
	}
	return a.telegramSendMessage(ctx, chatID, "邮件已发送。")
}

// splitRecipients 将逗号/分号/空白分隔的收件人拆分为规范地址列表。
func splitRecipients(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	var out []string
	seen := make(map[string]bool)
	for _, p := range parts {
		email := normalizeEmail(strings.TrimSpace(p))
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}
	return out
}

// telegramComposeCleanup 清理超过 maxAge 的会话，防止内存泄漏。
func (a *App) telegramComposeCleanup(maxAge time.Duration) {
	deadline := a.now().Add(-maxAge)
	a.telegramComposeMu.Lock()
	defer a.telegramComposeMu.Unlock()
	for chatID, sess := range a.telegramCompose {
		if sess == nil || sess.createdAt.Before(deadline) {
			delete(a.telegramCompose, chatID)
		}
	}
}
