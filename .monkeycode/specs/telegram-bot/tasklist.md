# Telegram Bot 实施任务列表

## 阶段 1：后端配置与数据库

- [x] 1.1 在 `config.go` 增加 Telegram 配置字段（`EOOS_TELEGRAM_BOT_TOKEN`、`EOOS_TELEGRAM_BOT_TIMEOUT_SECONDS`、`EOOS_TELEGRAM_ADMIN_CHAT_IDS`、`EOOS_TELEGRAM_NOTIFY_ENABLED`、`EOOS_TELEGRAM_ALERT_QUEUE_THRESHOLD`）
- [x] 1.2 更新 `deploy/.env.example` 增加 Telegram 环境变量示例
- [x] 1.3 SQLite 迁移新增 5 张表：`telegram_bindings`、`telegram_binding_codes`、`telegram_notify_outbox`、`telegram_mailbox_settings`、`telegram_alert_outbox`
- [x] 1.4 Postgres/MySQL schema 同步新增 5 张表

## 阶段 2：Bot 核心

- [x] 2.1 实现 `telegram_bot.go`：标准库长轮询 worker、Telegram API 客户端（sendMessage、sendPhoto、editMessageText、answerCallbackQuery）、offset 管理
- [x] 2.2 实现 `telegram_commands.go`：命令路由与 `/start`、`/status`、`/open`
- [x] 2.3 实现绑定流程：`/bind` 生成绑定码、`/unbind` 解绑、绑定码校验

## 阶段 3：交互邮件

- [x] 3.1 实现 `/inbox` 与 `/read <id>` 命令（inline keyboard）
- [x] 3.2 实现 `/send` 与 `/reply` 会话状态机，复用 `sendMailWithSource`

## 阶段 4：通知与告警

- [x] 4.1 实现 `telegram_notify.go`：`enqueueTelegramNotify` 入队（绑定/开关/spam 过滤）+ 通知出队列 worker + 指数退避
- [x] 4.2 在邮件落库路径（maildir Inbox、外部 IMAP、本地投递）挂钩通知入队
- [x] 4.3 实现 `telegram_alerts.go`：告警判定（队列失败/积压/worker 心跳/SMTP/DB）+ 告警出队列 worker

## 阶段 5：管理命令

- [x] 5.1 实现 `/admin` 命令组（users/domains/mailboxes/disable/enable）与 admin 权限校验

## 阶段 6：Web 设置与 API

- [x] 6.1 后端 API：`GET /api/me/telegram/settings`、`PUT /api/me/telegram/settings/{mailboxId}`、`GET /api/me/telegram/binding-code`
- [x] 6.2 前端 `lib/api.ts` 增加 Telegram 方法，设置页展示绑定码与邮箱通知开关

## 阶段 7：Mini App

- [x] 7.1 后端：`POST /api/telegram/webapp-auth` WebApp initData HMAC 校验并签发会话
- [x] 7.2 前端：`/telegram` 独立入口页，WebApp 静默登录与邮件详情定位

## 阶段 8：测试与验证

- [ ] 8.1 单元测试：命令路由、initData 校验、dedupe 键、退避计算、Markdown 转义
- [ ] 8.2 集成测试：通知入队过滤、管理命令权限、发送状态机（mock Telegram API）
- [ ] 8.3 全量 `go test ./...` 与前端构建验证
