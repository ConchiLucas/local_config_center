# 03 · 数据与服务端契约

## “读取数据库”的含义

推荐链路：浏览器 → 自己的后端只读接口 → 配置数据库。不是浏览器直连 PostgreSQL/MySQL/MinIO 并持有密码。数据库的驱动、连接与密钥解密都留在服务端。

配置读取和测试执行用不同权限：配置仓储账户只读；执行器只允许调用已登记的服务。测试记录需要写入时，写到独立运行存储，不修改配置库。

当前包的 SQLite 是替身，不是建议用户迁移数据库。已有统一 AI 配置时建立映射即可。`public_json` 演示结构仅存无敏感字段，生产环境必须显式白名单投影，不能把整行或任意 JSON 原样下发。

## 建议数据概念

`config_group`: id、displayName、sortOrder、enabled。

`config_page`: id、groupId、kind、displayName、rendererKey、schemaVersion、sortOrder。

`config_entry`: id、pageId、displayName、adapterKey、revision、publicSettings、secretReference、enabled。

`test_run`: id、configId、configRevision、请求者、输入摘要或受控存储引用、idempotencyKey、状态、上游 taskId、时间、指标、脱敏错误、取消状态。

`test_output`: runId、输出顺序、kind、文本/结构化数据或 artifactId、元信息。

在后端注册合法 `adapterKey`；在前端注册合法 `rendererKey`。数据库不得保存任意可执行 JS、远程组件地址、HTML 模板或 SQL 探针脚本直接下发运行。只通过受控 ID 选择已实现的组件。

## API 建议（生产契约，不表示原型全部实现）

| 方法与路径 | 用途 | 本原型 |
| --- | --- | --- |
| GET /api/catalog | 一级分组、二级目录与可公开配置 | 已实现，来自 SQLite |
| GET /api/configs/:id | 只读详情与版本 | 已实现 |
| POST /api/test-runs | 用配置 ID 和本次输入创建测试 | 明确返回 501，尚未接真实执行器 |
| GET /api/test-runs/:id | 状态、元信息、已完成输出 | 契约设计 |
| GET /api/test-runs/:id/events | SSE 或等效增量事件 | 契约设计 |
| POST /api/test-runs/:id/cancel | 请求取消，返回是否真正支持 | 契约设计 |
| GET /api/media/:assetId | 授权后的实际媒体 | 契约设计；原型用本地素材 |

本功能不存在配置 POST/PUT/PATCH/DELETE。创建测试是明确运行请求，不是新增配置，媒体控制按钮也不是配置操作按钮。

测试请求建议只包含：`configId`、`expectedRevision`、`input`、允许的本次测试选项、`idempotencyKey`。禁止前端传可替换的 baseURL、明文 API Key、数据库主机、任意 SQL 或任意目标 URL。

运行时后端按 ID 查配置，验证版本和访问权限，再选择 adapter。版本改变时返回明确的 CONFIG_CHANGED/409，让用户重新确认，不能静默使用不同版本。

## 模型适配与结果归一化

适配层至少拆成：`validateInput`、`start`、`observe/poll/stream`、可选 `cancel`、`normalizeOutput`。能力描述以实际服务和具体模型为准；OpenAI-compatible 等标签不足以保证所有接口、模式与响应类型完全一致。

流式文本：后端规范化 token/delta；客户端累加文本，UTF-8 和事件边界都要正确处理 [1]。不要假设一个网络 chunk 就是一个完整 SSE 事件；重连避免重复展示。

JSON：保留内容文本，完成后解析，报告合法性；启用特定结构约束时用服务端支持的 schema，并校验最终输出。失败时仍然允许看原始可显示内容，但标注解析失败。

媒体：先校验上游的结果类型、MIME、大小、输出归属；再登记 asset，并提供受控访问。HTML/SVG 等不可信主动内容要禁止脚本或转换成安全展示形式。原型不接受用户上传，仅展示包内可信素材。

## 安全与访问边界

API Key、数据库密码、MinIO secret key 不进入配置列表、浏览器、日志和错误详情。服务端使用 secret reference、最小权限和适当的安全存储；不能仅在网页中盖一排圆点。详见 [6]。

不显示登录入口 ≠ 公开无鉴权。局域网单人工具可在受控本地运行；生产或多人环境沿用已有会话、网关/反向代理认证、网络 ACL 与资源授权，不必在此页面另外设计登录框。

API 调用与媒体读取做目标白名单、协议/端口限制、重定向与 DNS 重新解析检查。支持本地数据库意味着可以访问明确登记且允许的私网服务，不是放开任意内网地址；云元数据等目标不能被测试功能访问。SSRF 防护依据 [5]。

数据库默认提供连接探针、SELECT 1；Redis 默认 PING。探针固定在服务端实现，采用只读/最小权限凭据和超时。MinIO 默认只读元信息、列举受限对象、读取受限样例，不自动写入或删除。

图片、语音、视频的真实测试可能收费。只在用户明确点击测试时执行，不因路由切换、自动刷新或页面打开而产生调用。生成失败重试要避免对计费任务无条件自动重复创建。

## 交付时必须真实说明

最终验收报告分别列出：读取了哪个数据源、调用了哪个配置版本、供应商是否真实调用、实际输出是否在页面中可播放、哪些项目未验证。模型能力和成本没有实测来源时保持未知，不填想当然的默认值。
