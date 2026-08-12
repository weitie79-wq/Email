# Telegram Bot 需求文档

## 引言

本特性为 EOOS Email 增加一个 Telegram Bot，支持新邮件推送通知、Telegram 内交互式收发邮件、管理员管理操作、以及服务告警监控。所有已登录用户可以各自绑定自己的 Telegram 账号，仅接收自己邮箱相关的通知。除 Bot 命令外，系统还提供一个 Telegram Mini App 入口，用户可在 Telegram 内直接打开邮件客户端界面。

## 术语表

- **Bot**：本特性提供的 Telegram 机器人服务，作为 EOOS Email 后端进程内组件运行。
- **绑定**：将一个 Telegram 聊天（用户）与一个 EOOS Email 用户账号关联的过程。
- **Mini App**：Telegram Web App，通过 Telegram 客户端内嵌打开 EOOS Email 前端页面。
- **绑定码**：用于完成绑定流程的一次性、限时短码。
- **通知项**：Bot 推送给已绑定用户的单条消息。
- **管理员**：拥有 `admin` 角色的 EOOS Email 用户。

## 需求

### 需求 1：Telegram 账号绑定

**用户故事：** 作为已登录用户，我想把我的 Telegram 账号与我的 EOOS Email 账号绑定，以便在 Telegram 中接收邮件通知和使用 Bot 功能。

#### 验收标准

1. WHEN 用户通过 Bot 发送 `/start` 命令，Bot SHALL 返回绑定引导信息。
2. WHEN 用户请求获取绑定码，Bot SHALL 生成一个有效期不超过 10 分钟的单次使用绑定码，并在该用户 EOOS Email 会话中展示绑定码。
3. WHEN 用户在已登录的 EOOS Email Web 界面中输入有效绑定码，系统 SHALL 将该 Telegram 聊天 ID 与该用户账号绑定，并将绑定结果通知 Bot。
4. WHEN 用户通过 Bot 发送 `/unbind` 命令，Bot SHALL 解绑当前聊天与 EOOS Email 账号的关联。
5. WHEN 一个 Telegram 聊天已绑定某一用户，Bot SHALL 拒绝该聊天再次绑定其他用户账号。
6. WHEN 一个 EOOS Email 用户已被某一 Telegram 聊天绑定，系统 SHALL 拒绝其他 Telegram 聊天绑定该用户账号。
7. WHEN 绑定码已过期、已使用或不存在，系统 SHALL 拒绝使用并提示用户重新生成。

### 需求 2：新邮件推送通知

**用户故事：** 作为已绑定用户，我想在收到新邮件时通过 Telegram 收到通知，以便及时了解新邮件。

#### 验收标准

1. WHEN 已绑定用户的一个收件箱邮箱收到新邮件，Bot SHALL 向该用户绑定的 Telegram 聊天推送通知。
2. 通知 SHALL 包含发件人、主题、摘要文本和收件时间。
3. 通知 SHALL 附带一个跳转按钮，WHEN 用户点击按钮，Telegram SHALL 打开指向该邮件详情的 Mini App。
4. WHEN 用户收到通知但绑定关系在通知生成后被解除，Bot SHALL 丢弃该通知而不再推送。
5. WHEN 新邮件进入垃圾邮件文件夹，Bot SHALL 不推送通知，除非该邮箱开启了垃圾邮件通知。
6. WHEN 推送失败（Telegram API 不可达），Bot SHALL 记录失败并按指数退避策略重试。
7. WHILE 用户已在 Web 界面阅读了该邮件，Bot SHALL 不因同一邮件重复生成通知。
8. WHEN 管理员在系统设置中禁用 Telegram 通知，Bot SHALL 停止所有新邮件通知推送。
9. 新邮件通知 SHALL 按邮箱级别开关控制；WHEN 某邮箱关闭通知开关，Bot SHALL 不向该邮箱的收件推送通知。
10. 邮箱通知开关 SHALL 可由用户在 Web 设置页中配置，默认开启。

### 需求 3：交互式收发邮件

**用户故事：** 作为已绑定用户，我想在 Telegram 内查看收件箱、阅读和发送邮件，以便在不离开 Telegram 的情况下处理邮件。

#### 验收标准

1. WHEN 用户通过 Bot 发送 `/inbox` 命令，Bot SHALL 返回该用户默认收件箱最近的邮件列表，每条包含发件人、主题和时间。
2. WHEN 用户从列表中选择一封邮件，Bot SHALL 返回该邮件的发件人、主题、时间、正文摘要和附件列表。
3. WHEN 用户通过 Bot 发送 `/send` 命令，Bot SHALL 进入引导式发信流程，依次收集收件人、主题和正文。
4. WHEN 发信流程完成且输入有效，Bot SHALL 通过现有邮件发送链路发送邮件，并返回发送结果。
5. WHEN 用户输入无效收件人地址，Bot SHALL 提示地址无效并要求重新输入。
6. WHEN 用户输入为空或长度超出系统限制，Bot SHALL 提示输入不合法并要求重新输入。
7. WHEN 发信请求提交时用户账号已解除绑定，Bot SHALL 拒绝发送并提示重新绑定。
8. WHEN 邮件正文包含 Markdown 特殊字符，Bot SHALL 转义后展示，防止 Telegram 消息注入。

### 需求 4：管理员操作

**用户故事：** 作为管理员，我想在 Telegram 中查看和管理用户、域名与邮箱，以便快速处理管理任务。

#### 验收标准

1. WHEN 管理员通过 Bot 发送 `/admin users` 命令，Bot SHALL 校验该聊天绑定账号的管理员角色，并在具备权限时返回用户列表。
2. WHEN 管理员通过 Bot 发送 `/admin domains` 命令，Bot SHALL 校验管理员角色，并在具备权限时返回域名列表及其状态。
3. WHEN 管理员通过 Bot 发送 `/admin mailboxes` 命令，Bot SHALL 校验管理员角色，并在具备权限时返回邮箱列表。
4. WHEN 管理员通过 Bot 发送 `/admin disable <用户>` 命令，Bot SHALL 校验管理员角色，并禁用目标用户账号。
5. WHEN 管理员通过 Bot 发送 `/admin enable <用户>` 命令，Bot SHALL 校验管理员角色，并启用目标用户账号。
6. WHEN 非管理员用户执行管理员命令，Bot SHALL 拒绝并提示无权限。
7. WHEN 管理员命令涉及对不存在的用户或域名，Bot SHALL 提示目标不存在。

### 需求 5：服务告警监控

**用户故事：** 作为管理员，我想通过 Telegram 接收系统告警，以便及时发现服务异常。

#### 验收标准

1. WHEN 发送队列中的邮件状态变为最终失败，Bot SHALL 向告警接收方推送失败告警。
2. WHEN 发送队列积压超过管理员配置的阈值，Bot SHALL 推送积压告警。
3. WHEN 关键后台 worker（发送队列、定时发送、状态 Webhook）连续心跳超时，Bot SHALL 推送 worker 异常告警。
4. WHEN SMTP 上游连接失败次数在时间窗口内超过阈值，Bot SHALL 推送 SMTP 连接异常告警。
5. WHEN 数据库连接失败，Bot SHALL 尝试推送数据库异常告警。
6. 告警 SHALL 包含告警类型、发生时间、受影响的组件和当前状态摘要。
7. 同一告警事件 SHALL 通过去重键避免在短时间内重复推送。
8. WHEN 管理员在系统设置中配置告警接收 Telegram 聊天，Bot SHALL 仅向该聊天推送告警。

### 需求 6：Telegram Mini App

**用户故事：** 作为已绑定用户，我想在 Telegram 内直接打开邮件客户端，以便获得完整的邮件体验。

#### 验收标准

1. WHEN 用户点击通知中的跳转按钮，Telegram SHALL 打开指向对应邮件详情的 Mini App。
2. WHEN 用户点击 Bot 菜单中的"打开客户端"按钮，Telegram SHALL 打开 Mini App 首页。
3. WHEN Mini App 加载，系统 SHALL 通过 Telegram WebApp 初始化数据校验用户身份，并在校验通过后自动登录对应用户会话。
4. WHEN Telegram WebApp 初始化数据校验失败或已过期，系统 SHALL 拒绝会话建立并引导用户在浏览器中登录。
5. Mini App SHALL 复用现有 EOOS Email Web 前端的组件与样式，作为独立精简入口构建。
6. WHEN 用户在 Mini App 中执行邮件操作，系统 SHALL 应用与 Web 客户端相同的权限与授权规则。
7. Mini App SHALL 仅在已绑定的 Telegram 会话中可用；WHEN 会话未绑定，系统 SHALL 提示先完成绑定。

### 需求 7：配置与部署

**用户故事：** 作为部署者，我想通过环境变量配置 Telegram Bot，以便按需启用。

#### 验收标准

1. WHEN 部署者设置 `EOOS_TELEGRAM_BOT_TOKEN`，系统 SHALL 启用 Telegram Bot 功能。
2. WHEN 部署者未设置 `EOOS_TELEGRAM_BOT_TOKEN`，系统 SHALL 不启动 Bot 且不影响邮件服务正常运行。
3. Bot 配置 SHALL 通过现有环境变量机制注入，并纳入 `.env.example` 文档。
4. Bot 组件 SHALL 作为 API 进程内的后台 worker 运行，无需新增独立进程或部署组件。
5. WHEN 用户邮件内容用于生成通知，系统 SHALL 仅发送摘要与必要元数据，不在 Telegram 中传输完整邮件原文。
6. Bot 与 Telegram API 之间的通信 SHALL 使用 HTTPS。

## 决策记录

- **Bot 实现方式**：使用标准库实现 Telegram `getUpdates` 长轮询，不引入第三方 Telegram 库。
- **通知粒度**：新邮件通知按邮箱级别开关控制，用户可在 Web 设置页为每个邮箱独立配置，默认开启。
- **Mini App**：构建独立的精简 Telegram Mini App 入口，复用现有 Web 前端的组件与样式。

## 参考资料

- 现有邮件落库汇合点：`apps/api/internal/app/mail_handlers.go` 的 `insertMessageWithDB`
- 事件出站队列表模式：`apps/api/internal/app/status_webhook.go`
- Worker 启动与退出模式：`apps/api/internal/app/app.go` 的 `startWorker`、`Close`
- 配置加载：`apps/api/internal/app/config.go` 的 `LoadConfig`
- 前端 API 客户端：`apps/web/src/lib/api.ts`
- 前端路由：`apps/web/src/main.tsx`
