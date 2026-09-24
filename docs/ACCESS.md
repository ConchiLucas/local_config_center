# 内部项目如何调用 local_config_center

给本机 Go / Python / Java 服务用。业务代码只打这个网关，不要写 Antigravity、Aurora、Grok、小米的 Key 或 Base URL。

网关地址默认 `http://127.0.0.1:18527`。常驻用 `/Users/conchi/docker-compose/dc.sh config-center up`，开发可用仓库里的 `make run`。密钥看页面右上角「网关密钥」，或环境变量 `GATEWAY_AUTH`，未改时是 `local-gateway`。

## 接口

| 方法 | 路径 | 作用 |
|---|---|---|
| POST | `/v1/invoke` | 调用模型 |
| GET | `/v1/models?auth=<密钥>` | 列出当前可调模型 |
| GET | `/v1/media/{id}` | 取本次返回的图片/音频 |

`POST /v1/invoke` 的 JSON：

| 字段 | 必填 | 说明 |
|---|---|---|
| `auth` | 是 | 与页面密钥相同 |
| `kind` | 是 | `chat` / `image` / `audio` / `transcribe` |
| `model` | 是 | `供应商/模型名`，见下方前缀 |
| `stream` | 否 | 仅 `chat` 为 true 时 SSE |
| `format` | 否 | `chat` 可设 `json` |
| `input` | 是 | 见 kind |

`model` 前缀：

| 前缀 | 用途 |
|---|---|
| `aurora/` | 对话、画图、语音 |
| `antigravity/` | 对话、生图 |
| `grok/` | 对话、生图、TTS、视频 |
| `mimo/` | 小米 TTS |

当前完整 slug 以 `GET /v1/models?auth=...` 为准，不要写死过期名单。

### input

**chat**

```json
{ "messages": [{ "role": "user", "content": "你好" }] }
```

**image**

```json
{ "prompt": "深海蓝控制台", "resolution": "2k", "aspect": "16x9" }
```

`resolution` / `aspect` 仅 Antigravity Pro 生图需要。`resolution`：`""` / `2k` / `4k`。`aspect`：`""` / `1x1` / `3x4` / `4x3` / `16x9` / `9x16` / `21x9`。

**audio（TTS）**

```json
{ "text": "你好", "voice": "冰糖" }
```

**transcribe**

```json
{ "filename": "a.wav", "audioBase64": "<base64>" }
```

### 响应

非流式：`{ "run": { "state", "outputs", "error" } }`。`state` 为 `succeeded` 时看 `outputs`：对话/听写是 `kind=text` 的 `text`；图片/音频是 `asset` / `assets` 里的 `access.path`（拼到网关根路径请求）。失败时 `error.safeMessage` 可读，不要把厂商密钥记进日志。

鉴权失败 HTTP 401。

## 示例

密钥用 `local-gateway` 仅作占位，以页面为准。

### curl

```sh
curl -s http://127.0.0.1:18527/v1/invoke \
  -H 'Content-Type: application/json' \
  -d '{
    "auth": "local-gateway",
    "kind": "chat",
    "model": "aurora/auto",
    "input": {"messages":[{"role":"user","content":"只回复两个字：成功"}]}
  }'
```

### Go

```go
type invokeReq struct {
	Auth  string         `json:"auth"`
	Kind  string         `json:"kind"`
	Model string         `json:"model"`
	Input map[string]any `json:"input"`
}
body, _ := json.Marshal(invokeReq{
	Auth:  "local-gateway",
	Kind:  "chat",
	Model: "aurora/auto",
	Input: map[string]any{"messages": []map[string]string{{"role": "user", "content": "你好"}}},
})
resp, err := http.Post("http://127.0.0.1:18527/v1/invoke", "application/json", bytes.NewReader(body))
```

### Python

```python
import json, urllib.request
req = urllib.request.Request(
    "http://127.0.0.1:18527/v1/invoke",
    data=json.dumps({
        "auth": "local-gateway",
        "kind": "chat",
        "model": "aurora/auto",
        "input": {"messages": [{"role": "user", "content": "你好"}]},
    }).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
with urllib.request.urlopen(req, timeout=120) as r:
    print(json.load(r))
```

### Java

```java
var body = """
  {"auth":"local-gateway","kind":"chat","model":"aurora/auto",
   "input":{"messages":[{"role":"user","content":"你好"}]}}
  """;
var client = HttpClient.newHttpClient();
var request = HttpRequest.newBuilder()
    .uri(URI.create("http://127.0.0.1:18527/v1/invoke"))
    .header("Content-Type", "application/json")
    .POST(HttpRequest.BodyPublishers.ofString(body))
    .build();
var response = client.send(request, HttpResponse.BodyHandlers.ofString());
```

## 持久化异步生图任务（Batch Worker）

新增接口全部要求 `Authorization: Bearer <网关密钥>`，包括查询与图片下载。不要把密钥放进 URL、任务记录或前端。旧 `/v1/invoke` 合同保持不变。

| 方法 | 路径 | 作用 |
|---|---|---|
| GET | `/v1/image-jobs/capabilities` | 免费确认鉴权及协议，返回 `{version:1,real_enabled:false,parameters:"configured_defaults"}` |
| POST | `/v1/image-jobs` | 接受并持久化任务，HTTP 202 |
| GET | `/v1/image-jobs/{id}` | 查询任务 |
| GET | `/v1/image-jobs/by-key/{key}` | 按 URL 编码的幂等键恢复查询 |
| GET | `/v1/image-jobs/{id}/image` | 下载成功任务的持久化图片；仍要求 Bearer 鉴权 |

提交必须带 `Idempotency-Key`（非空，最多 200 字节），请求体：

```json
{"model":"mock/image","prompt":"深海蓝控制台","parameters":{}}
```

`parameters` 必须是空对象；当前只使用网关已配置的默认参数，不支持尺寸覆盖。提示词最多 20000 个 Unicode 字符；请求体最多 128 KiB。持久化图片只支持 PNG / JPEG / WebP，最多 20 MiB。同键同内容返回同一个任务；不同内容返回 409。提交连接失败时，先按原键查询或使用**同键同内容**重新提交，不能自行换键调用。

```json
{"job":{"id":"job-...","model":"mock/image","state":"succeeded","result":{"path":"/v1/image-jobs/job-.../image","media_type":"image/png"},"billed_amount":"0"}}
```

状态为 `queued` / `running` / `succeeded` / `failed` / `unknown`。错误结构为 `{code,message,outcome_known,retryable}`。`unknown` 表示上游结果或计费无法确认，**不得自动重试**。真实调用的 `billed_amount` 永远是 `null`（尚无可靠计费来源），只有本地 `mock/image` 是字符串 `"0"`。Mock 按提示词生成确定性 PNG，不调用供应商。

真实新接口默认关闭。管理员明确启用 `GATEWAY_IMAGE_REAL_ENABLED=true` 并重启后，才可提交来自现有 `/v1/models` 的完整 image 模型 slug。该开关只影响新的任务接口，不改变原 `/v1/invoke`。鉴权成功、模型可见或凭据存在都不代表授权付费生图。

任务、幂等键、状态与图片 bytes 存于当前 `DATABASE_URL` 指向的 PostgreSQL 的独立 `image_jobs.jobs` 表，不写 `config_documents`。数据库角色需能创建 `image_jobs` schema / 表并读写自身对象。备份应包含该 schema。图片和成功状态在同一次数据库更新中提交，重启后不依赖旧内存媒体索引。当前不自动清理任务或幂等键；不要独立删除仍需恢复的记录。

每个数据库通过专用会话 advisory lock 只允许一个新任务执行器；第二个实例启动失败。不要经事务池模式的 PgBouncer 连接这个执行器（需要稳定 PostgreSQL 会话）。连接丢失时执行器停止服务，不能在未持锁的新连接上继续执行。新实例恢复时，已 `running` 任务统一转为 `unknown` 且不会重发；`queued` 任务继续执行，完成写入还会核对 owner，防止旧实例覆盖恢复状态。

测试使用**专用可丢弃数据库**：设置 `IMAGE_JOBS_TEST_DATABASE_URL` 后从 `server` 目录运行 `go test -race ./...`。集成测试会清空该测试数据库的 `image_jobs.jobs`，不可指向正式配置中心数据库。测试只调用 Mock，不触发付费供应商。
