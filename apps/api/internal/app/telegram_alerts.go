package app

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// telegramAlertTypes 定义告警类型。
const (
	telegramAlertQueueBacklog = "queue_backlog"
	telegramAlertSMTP         = "smtp"
	telegramAlertDB           = "db"
	telegramAlertWorker       = "worker"
	telegramAlertHeartbeat    = "heartbeat"
)

// enqueueTelegramAlert 将告警写入 telegram_alert_outbox，按 dedupe_key 去重。
func (a *App) enqueueTelegramAlert(ctx context.Context, alertType, title, body string) error {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return nil
	}
	chatID, err := a.telegramAlertTargetChatID(ctx)
	if err != nil {
		return err
	}
	if chatID == 0 {
		return nil
	}
	dedupeKey := alertType + ":" + a.now().UTC().Format("2006-01-02T15:04")
	id := newID("ta")
	now := a.now().UTC().Format(time.RFC3339Nano)
	query := insertIgnoreSQL(a.cfg.DBDriver,
		`INSERT INTO telegram_alert_outbox(id, alert_type, chat_id, title, body, dedupe_key,
		   attempt_count, max_attempts, next_attempt_at, delivered_at, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		`(dedupe_key)`)
	_, err = a.db.ExecContext(ctx, query,
		id, alertType, chatID, title, body, dedupeKey,
		0, 5, now, "", now)
	return err
}

// telegramAlertTargetChatID 返回告警接收 chat：优先绑定 is_admin_target=1 的聊天，
// 否则取配置的 TelegramAdminChatIDs，最后取任一绑定聊天。
func (a *App) telegramAlertTargetChatID(ctx context.Context) (int64, error) {
	var chatID int64
	err := a.db.QueryRowContext(ctx, `SELECT chat_id FROM telegram_bindings WHERE is_admin_target=1 ORDER BY created_at LIMIT 1`).Scan(&chatID)
	if err == nil && chatID != 0 {
		return chatID, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}
	for _, adminChatIDStr := range strings.Split(a.cfg.TelegramAdminChatIDs, ",") {
		adminChatID, parseErr := strconv.ParseInt(strings.TrimSpace(adminChatIDStr), 10, 64)
		if parseErr == nil && adminChatID != 0 {
			return adminChatID, nil
		}
	}
	err = a.db.QueryRowContext(ctx, `SELECT chat_id FROM telegram_bindings ORDER BY created_at LIMIT 1`).Scan(&chatID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return chatID, err
}

// checkTelegramQueueBacklog 检查 send_queue 中滞留的失败/超时任务。
func (a *App) checkTelegramQueueBacklog(ctx context.Context) {
	threshold := a.cfg.TelegramAlertQueueThreshold
	if threshold <= 0 {
		return
	}
	var staleCount, failedCount int
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM send_queue WHERE status='queued' AND next_attempt_at<=?`,
		a.now().UTC().Format(time.RFC3339Nano)).Scan(&staleCount)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM send_queue WHERE status='failed'`).Scan(&failedCount)
	if staleCount+failedCount >= threshold {
		_ = a.enqueueTelegramAlert(ctx, telegramAlertQueueBacklog,
			fmt.Sprintf("发件队列积压 (≥%d)", threshold),
			fmt.Sprintf("滞留队列: %d 条，失败: %d 条。请检查 SMTP 配置与网络。", staleCount, failedCount))
	}
}

// checkTelegramSMTPHealth 检查最近 SMTP 投递失败率。
func (a *App) checkTelegramSMTPHealth(ctx context.Context) {
	now := a.now().UTC()
	since := now.Add(-30 * time.Minute).Format(time.RFC3339Nano)
	var failed, delivered int
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM send_audit_events WHERE event='failed' AND created_at>=?`, since).Scan(&failed)
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM send_audit_events WHERE event='delivered' AND created_at>=?`, since).Scan(&delivered)
	if failed >= 5 && delivered == 0 {
		_ = a.enqueueTelegramAlert(ctx, telegramAlertSMTP,
			"SMTP 投递异常",
			fmt.Sprintf("近 30 分钟失败 %d 次、成功 0 次，SMTP 可能不可用。", failed))
	}
}

// checkTelegramDBHealth 检查数据库基本健康状态（写入探测）。
func (a *App) checkTelegramDBHealth(ctx context.Context) {
	if err := a.db.PingContext(ctx); err != nil {
		_ = a.enqueueTelegramAlert(ctx, telegramAlertDB, "数据库异常", "数据库 PING 失败: "+err.Error())
		return
	}
}

// checkTelegramAlerts 执行一轮告警判定。
func (a *App) checkTelegramAlerts(ctx context.Context) {
	if strings.TrimSpace(a.cfg.TelegramBotToken) == "" {
		return
	}
	if a.cfg.TelegramAdminChatIDs == "" {
		// 未配置管理员 chat 时，检查是否已有管理员绑定，避免向未知聊天发送告警
		var hasTarget int
		_ = a.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM telegram_bindings WHERE is_admin_target=1`).Scan(&hasTarget)
		if hasTarget == 0 {
			return
		}
	}
	a.checkTelegramQueueBacklog(ctx)
	a.checkTelegramSMTPHealth(ctx)
	a.checkTelegramDBHealth(ctx)
}
