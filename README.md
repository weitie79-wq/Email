# EOOS Email

[English](./README.md)

简体中文版

---

> EOOS Email 是一个自建邮箱 Webmail 全栈方案。
> 前端：React + TypeScript + shadcn/ui
> 后端：Go + SQLite
> 部署：Docker（单容器 all-in-one / 多容器 stack）
> 社区：Telegram [群](https://t.me/+EhII7MSyi3QwNDQ5)

## 功能特性

- **Webmail 客户端**：多邮箱切换、文件夹、读写草稿、定时发送、附件、搜索、标签、星标、移动/删除、已读/未读
- **邮箱增强**：联系人、签名、收件规则、发件人黑名单、邮件统计、归档已读、清空回收站/垃圾邮件
- **多域名/多邮箱**：域名管理、DKIM 密钥生成、DNS 记录展示与检测、邮箱账号、别名转发、无人收件开关
- **账号与权限**：登录/注册、会话管理、TOTP 两步验证、Cloudflare Turnstile、用户自助申请邮箱、权限组/RBAC
- **管理员面板**：概览清单、用户/权限组/域名/邮箱/别名/全部邮件管理、系统设置、邮件模板、SMTP 测试
- **邮件服务栈**：Postfix 投递、Dovecot IMAP/POP3、Rspamd 反垃圾与 DKIM 签名、Maildir 到 SQLite 同步
- **部署友好**：默认 all-in-one 单容器，也提供多容器 stack 方便调试 Postfix/Dovecot/Rspamd

## Telegram Bot 集成

为 EOOS Email 集成 Telegram Bot 支持，提供：

- 新邮件推送通知
- 交互式邮件收发（/inbox /read /send /reply）
- 管理员操作（/admin users /admin domains /admin mailboxes）
- 服务告警监控

### 配置步骤

1. 创建 `.env` 文件：

```bash
cp deploy/.env.example deploy/.env
```

2. 编辑 `deploy/.env`：

```bash
EOOS_TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
EOOS_PUBLIC_BASE_URL=https://your-domain.com
```

3. 启动后端：

```bash
cd apps/api
source .env
go run ./cmd/server
```

4. 在 Telegram 中添加 Bot → `/setmenubutton` → 选择 `Use it to create a custom menu` → 填入 API URL

#### 页面使用

打开 Telegram → 点击 Bot 菜单 → 选择「打开客户端」→ Mini App 加载完成，跳转至邮件列表。

## 快速开始

### 本地开发

```bash
# 后端
cd apps/api
go mod download
go test ./...
go run ./cmd/server

# 前端
cd apps/web
corepack enable
corepack prepare pnpm@10.28.2 --activate
pnpm install
pnpm run dev
```

访问：

- Web：`http://localhost:5173`
- API：`http://localhost:8080`

默认管理员邮箱为 `admin@eoos.local`。建议开发时显式设置 `EOOS_ADMIN_PASSWORD`；如果未设置，后端首次启动会随机生成密码并输出到日志。

### Docker 部署

服务器只需要 `deploy/` 下的 Compose 文件和配置，不需要源码构建：

```bash
cd deploy
cp .env.example .env
# 编辑 .env: 域名、Public URL、管理员邮箱、密码等
docker compose pull
docker compose up -d
```

常见命令：

```bash
# 查看日志
docker compose logs -f eoos-email

# 重建镜像
docker compose pull
docker compose up -d --build
```

## 部署清单

| 组件 | 镜像 | 端口 |
|------|------|------|
| All-in-one | `ghcr.io/eoos996/eoos-email:latest` | 80, 443, 25, 465, 587, 993, 995 |
| API | `ghcr.io/eoos996/eoos-email-api:latest` | 8080 |
| Web | `ghcr.io/eoos996/eoos-email-web:latest` | 5173 |
| Postfix | `ghcr.io/eoos996/eoos-email-postfix:latest` | 25 |
| Dovecot | `ghcr.io/eoos996/eoos-email-dovecot:latest` | 993, 995 |
| Rspamd | `ghcr.io/eoos996/eoos-email-rspamd:latest` | 11332 |

## 关键环境变量

见 [`deploy/.env.example`](./deploy/.env.example) 获取完整配置。

### Telegram Bot

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `EOOS_TELEGRAM_BOT_TOKEN` | 从 @BotFather 获取的 Bot Token | 空 |
| `EOOS_TELEGRAM_BOT_TIMEOUT_SECONDS` | 长轮询超时 | 60 |
| `EOOS_TELEGRAM_ADMIN_CHAT_IDS` | 告警接收 Chat ID，逗号分隔 | 空 |
| `EOOS_TELEGRAM_NOTIFY_ENABLED` | 全局通知开关 | true |
| `EOOS_TELEGRAM_ALERT_QUEUE_THRESHOLD` | 积压告警阈值 | 200 |
| `EOOS_PUBLIC_BASE_URL` | Mini App 入口域名 | `http://localhost:5173` |

## 快速开始清单

1. **编辑 `.env`**：设置 `EOOS_PUBLIC_HOSTNAME`、`EOOS_PUBLIC_BASE_URL`、`EOOS_ADMIN_EMAIL`、`EOOS_ADMIN_PASSWORD`
2. **启动后端**：`cd apps/api && source .env && go run ./cmd/server`
3. **启动 Web**：`cd apps/web && corepack enable && pnpm install && pnpm run dev`
4. **查看 Web**：`http://localhost:5173`
5. **管理员面板**：登录后添加邮件域名，完成 MX/SPF/DKIM/DMARC 记录

## Troubleshooting

- `401 Unauthorized`：Telegram Bot Token 无效。生成 Token：`@BotFather → /newbot → 设置 Bot Token`
- `503 Service Unavailable`：Telegram Bot 未配置。添加 `EOOS_TELEGRAM_BOT_TOKEN` 环境变量。
- `404 page not found`：端口未开放。检查防火墙或 Nginx 配置。
- `maildir sync` 慢：检查 `EOOS_MAILDIR_SCAN_SECONDS`，建议 30 秒。
- 后端日志：`docker compose logs -f eoos-email`（单容器）或 `docker compose logs -f eoos-email-api`（多容器）。

## 参考

- [Telegram Mini App 入口设计](./.monkeycode/specs/telegram-bot/design.md)
- [Telegram Bot 需求文档](./.monkeycode/specs/telegram-bot/requirements.md)
- [API 文档](./docs/API.md)
- [Postfix 配置](./deploy/postfix/)
- [Dovecot 配置](./deploy/dovecot/)
- [Rspamd 配置](./deploy/rspamd/)
