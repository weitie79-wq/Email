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
- **Telegram Bot**：新邮件通知、交互式邮件收发、管理命令、Mini App 入口
- **部署友好**：默认 all-in-one 单容器，也提供多容器 stack 方便调试 Postfix/Dovecot/Rspamd

---

## 安装步骤

### 第一步：克隆仓库

```bash
git clone https://github.com/eoos996/EOOS-Email.git
cd EOOS-Email
```

### 第二步：配置环境变量

```bash
cp deploy/.env.example deploy/.env
```

编辑 `deploy/.env`，至少修改以下变量：

```bash
# 邮件服务器域名（用于 Postfix/MX/DKIM）
EOOS_PUBLIC_HOSTNAME=mail.your-domain.com

# Web 访问地址
EOOS_PUBLIC_BASE_URL=https://mail.your-domain.com

# 初始管理员账号
EOOS_ADMIN_EMAIL=admin@your-domain.com
EOOS_ADMIN_PASSWORD=YourStrongPassword123

# Telegram Bot（可选）
# 从 @BotFather 获取 Token：https://t.me/BotFather
EOOS_TELEGRAM_BOT_TOKEN=
```

### 第三步：启动服务

#### 方式一：Docker 快速部署（推荐）

```bash
cd deploy
docker compose up -d
```

查看日志：

```bash
docker compose logs -f eoos-email
```

#### 方式二：本地开发部署

1. 安装依赖

```bash
# Go 后端
cd apps/api
go mod download

# Node 前端
cd ../../apps/web
corepack enable
corepack prepare pnpm@10.28.2 --activate
pnpm install
```

2. 启动后端（新终端）

```bash
cd apps/api
source ../deploy/.env 2>/dev/null || true
go run ./cmd/server
```

3. 启动前端（新终端）

```bash
cd apps/web
pnpm run dev
```

访问地址：

| 服务 | 地址 |
|------|------|
| Webmail | http://localhost:5173 |
| API | http://localhost:8080 |
| 管理后台 | http://localhost:5173/admin |

---

## Telegram Bot 配置

### 获取 Bot Token

1. 打开 Telegram，搜索 [@BotFather](https://t.me/BotFather)
2. 发送 `/newbot` 命令
3. 按提示输入 Bot 名称和用户名
4. BotFather 会返回一个 Token，格式类似：`1234567890:ABCdefGHIjklMNOpqrsTUVwxyz`

### 配置 Mini App 菜单按钮

1. 在 BotFather 中发送 `/setmenubutton`
2. 选择你的 Bot
3. 选择 `Use it to create a custom menu`
4. 输入 Mini App URL：`https://mail.your-domain.com/telegram`

### 在 EOOS 中启用

在 `deploy/.env` 中配置：

```bash
EOOS_TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
EOOS_TELEGRAM_NOTIFY_ENABLED=true
EOOS_TELEGRAM_ADMIN_CHAT_IDS=
```

然后重启后端：

```bash
# Docker
docker compose up -d --force-recreate eoos-email

# 本地
pkill -f "go run ./cmd/server" && source deploy/.env && go run ./cmd/server
```

### Telegram Bot 功能

| 命令 | 说明 |
|------|------|
| `/start` | 绑定 Telegram 账号 |
| `/bind` | 获取绑定码 |
| `/unbind` | 解绑账号 |
| `/status` | 查看绑定状态 |
| `/inbox` | 查看收件箱 |
| `/read <id>` | 阅读邮件 |
| `/send` | 发送邮件 |
| `/reply` | 回复邮件 |
| `/admin users` | 列出用户 |
| `/admin domains` | 列出域名 |
| `/admin mailboxes` | 列出邮箱 |
| `/admin disable <email>` | 禁用用户 |
| `/admin enable <email>` | 启用用户 |

---

## 环境变量参考

完整配置见 [`deploy/.env.example`](./deploy/.env.example)。

### 核心配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `EOOS_PUBLIC_HOSTNAME` | 邮件服务器主机名 | `mail.example.com` |
| `EOOS_PUBLIC_BASE_URL` | Web 访问地址 | `https://mail.example.com` |
| `EOOS_ADMIN_EMAIL` | 初始管理员邮箱 | `admin@example.com` |
| `EOOS_ADMIN_PASSWORD` | 初始管理员密码 | 随机生成 |
| `EOOS_DB_DRIVER` | 数据库驱动 | `sqlite` |
| `EOOS_DB_PATH` | 数据库文件路径 | `/data/eoos.db` |
| `EOOS_SMTP_HOST` | SMTP 主机 | `127.0.0.1` |
| `EOOS_SMTP_PORT` | SMTP 端口 | `25` |
| `EOOS_MAILDIR_ROOT` | Maildir 根目录 | `/var/mail/vhosts` |

### Telegram Bot 配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `EOOS_TELEGRAM_BOT_TOKEN` | Bot Token（必填） | 空 |
| `EOOS_TELEGRAM_BOT_TIMEOUT_SECONDS` | 长轮询超时 | 60 |
| `EOOS_TELEGRAM_ADMIN_CHAT_IDS` | 告警接收 Chat ID | 空 |
| `EOOS_TELEGRAM_NOTIFY_ENABLED` | 全局通知开关 | true |
| `EOOS_TELEGRAM_ALERT_QUEUE_THRESHOLD` | 积压告警阈值 | 200 |

---

## 部署清单

| 组件 | 镜像 | 端口 |
|------|------|------|
| All-in-one | `ghcr.io/eoos996/eoos-email:latest` | 80, 443, 25, 465, 587, 993, 995 |
| API | `ghcr.io/eoos996/eoos-email-api:latest` | 8080 |
| Web | `ghcr.io/eoos996/eoos-email-web:latest` | 5173 |
| Postfix | `ghcr.io/eoos996/eoos-email-postfix:latest` | 25 |
| Dovecot | `ghcr.io/eoos996/eoos-email-dovecot:latest` | 993, 995 |
| Rspamd | `ghcr.io/eoos996/eoos-email-rspamd:latest` | 11332 |

---

## 首次部署检查清单

1. 编辑 `deploy/.env`：设置 `EOOS_PUBLIC_HOSTNAME`、`EOOS_PUBLIC_BASE_URL`、`EOOS_ADMIN_EMAIL`、`EOOS_ADMIN_PASSWORD`
2. 启动服务：`cd deploy && docker compose up -d`
3. 浏览器访问 Webmail：`http://localhost:5173`
4. 登录管理后台，添加邮件域名
5. 复制 MX/SPF/DKIM/DMARC 记录到 DNS 配置
6. 使用 DMARC 检测工具验证 DNS 记录

---

## Troubleshooting

- **Telegram 无法推送通知**：检查 `EOOS_TELEGRAM_BOT_TOKEN` 是否正确，确认 Bot 未被封禁
- **Mini App 无法加载**：确认 `EOOS_PUBLIC_BASE_URL` 指向可访问的地址
- **401 Unauthorized**：Telegram Bot Token 无效，重新从 BotFather 获取
- **503 Service Unavailable**：Telegram Bot 未配置或未启用
- **端口冲突**：检查防火墙或 Nginx 配置
- **maildir sync 慢**：调整 `EOOS_MAILDIR_SCAN_SECONDS`，建议 30 秒

---

## 参考

- [Telegram Mini App 入口设计](./.monkeycode/specs/telegram-bot/design.md)
- [Telegram Bot 需求文档](./.monkeycode/specs/telegram-bot/requirements.md)
- [API 文档](./docs/API.md)
- [部署文档](./deploy/README.md)
