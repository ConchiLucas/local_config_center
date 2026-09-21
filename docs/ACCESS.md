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
