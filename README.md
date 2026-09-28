# local_config_center

本机配置中心：只读展示 + 多媒体测试工作台 + 内部网关。

读取 `ai_share_config` 正在使用的 PostgreSQL 库 `shared_config`，把已有配置投影成顶部一级菜单和左侧二级目录。页面只展示、只测试，不提供新增、编辑、删除或保存。

测试由本服务后端执行：浏览器不会拿到 API Key、数据库密码或 MinIO 密钥。结果在右侧工作台直接阅读、放大或播放。失败就显示失败，不会用演示素材顶替。

## 启动

确保本机 `postgres16` 容器已启动，且存在 `shared_config` 库（与 Shared Config Center 相同）。浏览器打开 <http://127.0.0.1:18527>。常驻网关和开发进程不要同时占用 18527。

### 常驻（推荐）

编排在 `/Users/conchi/docker-compose/config-center/`，和其它本机容器一样走 `dc.sh`：

```sh
make docker-bin
cd /Users/conchi/docker-compose
./dc.sh config-center build
./dc.sh config-center up
```

容器名 `local-config-center`，映射 `18527:18527`，接入 `vibedeploy-shared`。改 Go 代码后同样先 `make docker-bin`，再 `build` + `up`。`dc.sh` 的 `up` 不会带 `--build`。

### 开发

```sh
make run
```

或：

```sh
cd server
DATABASE_URL='postgres://shared_config:shared_config_internal@127.0.0.1:5432/shared_config?sslmode=disable' \
HTTP_ADDR='127.0.0.1:18527' \
WEB_DIR='../web' \
ARTIFACT_DIR='../data/artifacts' \
go run ./cmd/workbench
```

宿主机进程默认把 `host.docker.internal` 映射到 `127.0.0.1`，把 `grok2api:8000`、`sub2api:8090` 和 `minio:9000` 分别映射到宿主机的 `127.0.0.1:37821`、`127.0.0.1:8090` 和 `127.0.0.1:19100`。容器内已设置 `HOST_SIDE_REWRITES=0`，Postgres 走 `host.docker.internal:5432`，Grok2API、Sub2API 和 MinIO 直接走 `vibedeploy-shared` 共享网络上的容器服务名。

## 内部网关

Go / Python / Java 如何调用见 [docs/ACCESS.md](docs/ACCESS.md)。给其它仓库里 AI 用的说明在用户 skill `local-llm-gateway`。

## 边界

- 配置真源仍是 `config_documents`，本服务只读。
- `POST /api/test-runs` 是运行请求，不是写配置。
- 数据库探针仅 `连接` / `SELECT 1` / `PING`。
- MinIO 只列举和读取已登记桶与前缀。
- 本地 CLI 只检查可执行文件是否存在，不代跑完整任务。
