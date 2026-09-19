# local_config_center

本机配置中心：只读展示 + 多媒体测试工作台 + 内部网关。

读取 `ai_share_config` 正在使用的 PostgreSQL 库 `shared_config`，把已有配置投影成顶部一级菜单和左侧二级目录。页面只展示、只测试，不提供新增、编辑、删除或保存。

测试由本服务后端执行：浏览器不会拿到 API Key、数据库密码或 MinIO 密钥。结果在右侧工作台直接阅读、放大或播放。失败就显示失败，不会用演示素材顶替。

## 启动

确保本机 `postgres16` 容器已启动，且存在 `shared_config` 库（与 Shared Config Center 相同）。

```sh
cd server
DATABASE_URL='postgres://shared_config:shared_config_internal@127.0.0.1:5432/shared_config?sslmode=disable' \
HTTP_ADDR='127.0.0.1:18527' \
WEB_DIR='../web' \
ARTIFACT_DIR='../data/artifacts' \
go run ./cmd/workbench
```

或在仓库根目录执行 `make run`。浏览器打开 <http://127.0.0.1:18527>。

默认把 `host.docker.internal` 映射到 `127.0.0.1`，把 `minio:9000` 映射到 `127.0.0.1:19100`，以便本进程跑在宿主机时能打到本地容器。若在 compose 网络内运行，设置 `HOST_SIDE_REWRITES=0`。

## 内部网关

本机项目可 `POST /v1/invoke`，JSON 里带 `auth` 即可，不传各厂商密钥。默认鉴权串是 `local-gateway`，可用环境变量 `GATEWAY_AUTH` 改掉。

```sh
curl -s http://127.0.0.1:18527/v1/invoke \
  -H 'Content-Type: application/json' \
  -d '{
    "auth": "local-gateway",
    "kind": "chat",
    "model": "antigravity/gemini-3.8-flash-high",
    "input": {"messages":[{"role":"user","content":"只回复两个字：成功"}]}
  }'
```

生图可带 `resolution`、`aspect`。模型清单：`GET /v1/models?auth=local-gateway`。音频/图片走 `/v1/media/{id}`。

## 边界

- 配置真源仍是 `config_documents`，本服务只读。
- `POST /api/test-runs` 是运行请求，不是写配置。
- 数据库探针仅 `连接` / `SELECT 1` / `PING`。
- MinIO 只列举和读取已登记桶与前缀。
- 本地 CLI 只检查可执行文件是否存在，不代跑完整任务。
