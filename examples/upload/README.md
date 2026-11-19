# 原图上传示例

## 功能一览

| 功能与场景 | 操作 | 页面预期结果 |
| --- | --- | --- |
| [上传原图](midscene/upload-original.yaml) | 以测试用户登录，选择「山景.jpg」，上传后从「我的文件」下载。 | 「我的文件」中能看到「山景.jpg」，并显示「已下载 山景.jpg」 |
| [再次找到文件](midscene/find-after-refresh.yaml) | 以测试用户登录并上传一张照片，点「刷新页面」，然后下载原文件。 | 刷新后「我的文件」中仍能找到「山景.jpg」，并显示「已下载 山景.jpg」 |
| [未登录时上传](midscene/guest-upload.yaml) | 以访客身份打开页面，选择「山景.jpg」并尝试上传。 | 页面显示「请先登录再继续。」，上传区域没有「上传成功」提示 |

每个场景都从独立的浏览器会话开始。「再次找到文件」会先上传自己的测试文件，可以单独运行。固定测试图片是 [fixtures/山景.jpg](fixtures/山景.jpg)。

## 从干净检出启动

需要 Docker 和 Compose v2。在 kratos-contrib 仓库根目录运行以下命令。

```sh
cd examples/upload
docker compose up --build -d --wait
```

打开 [原图上传示例](http://127.0.0.1:8080/)，点击「以测试用户登录」。用上面的测试图片体验上传、刷新和下载。测试会话由本地示例签发，上传记录保存在 SQLite，文件保存在私有 MinIO 桶。浏览器直接向 MinIO 发出带签名的 PUT 和 GET 请求。

Compose 默认绑定本机的 `8080`（页面与 API）和 `9000`（MinIO S3 API）。如果端口被占用，启动前设置 `UPLOAD_HTTP_PORT` 和 `UPLOAD_S3_PORT`，例如 `18724` 和 `19724`。停止服务用 `docker compose down`。默认的数据卷会保留，用同一项目重新启动后可以继续查看已上传的文件。

## 运行 Midscene 并查看报告

在 `examples/upload` 目录，需要 Node.js 24 和可调用的视觉模型。先安装锁定的依赖和 Chromium。

```sh
npm ci --ignore-scripts
npx playwright install chromium
```

模型配置可以放在当前 shell 或该目录的 `.env`。

| 变量 | 值 |
| --- | --- |
| `MIDSCENE_MODEL_BASE_URL` | 模型服务的 API 基址，例如 `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `MIDSCENE_MODEL_API_KEY` | 模型服务密钥 |
| `MIDSCENE_MODEL_NAME` | 已开通的视觉模型，例如 `qwen3.7-plus` |
| `MIDSCENE_MODEL_FAMILY` | Midscene 对应模型系列，例如 `qwen3` |

示例启动后运行以下命令。

```sh
npm run midscene
```

可以用 `npm run midscene -- find-after-refresh` 只运行一条故事。若更改了页面端口，同时把 `UPLOAD_BASE_URL` 设为页面地址，例如 `http://127.0.0.1:18724`。场景锁定到官方 `@midscene/cli@1.18.1` 的 YAML runner。模型和 YAML 配置参见 [Midscene 模型配置](https://midscenejs.com/model-api-setup)和 [YAML 运行指南](https://midscenejs.com/automate-with-scripts-in-yaml)。

运行后直接用浏览器打开 `midscene_run/report/` 中的 HTML 文件。各场景的实际报告文件名见 `midscene_run/results.json`，中文结果见 `midscene_run/summary.md`。GitHub Actions 的 **upload-midscene** artifact 保存这些报告和结果。CI 使用同名仓库变量配置模型，其中 `MIDSCENE_MODEL_API_KEY` 使用 Actions secret。模型配置缺失时，结果摘要会逐项标明「未运行」。

## 证据与自动化测试

| 结论 | 证据 |
| --- | --- |
| 测试用户可以选图、上传、刷新后找到原图；访客看到登录提示 | Midscene 对页面可见内容做断言。Playwright 也覆盖这三条故事，可以在没有模型的环境运行。 |
| 浏览器下载的是原文件 | Playwright 用浏览器实际保存的文件与固定图片做 SHA-256 比对。 |
| 私有桶拒绝匿名读取，签名 PUT 和 GET 可执行 | Go E2E 直接向 MinIO 发请求，要求匿名 GET 返回 HTTP 403，并对签名 GET 下载内容做 SHA-256 比对。 |
| 重复和并发确认返回同一结果 | Go E2E 同时发送八次确认，再重复确认；业务测试覆盖较晚到达的失败校验、大小不符、归属和存储临时失败后的重试。 |
| 缺失或无效身份被拒绝 | Go E2E 检查所有上传 API 的 HTTP 401；Supabase 模块测试覆盖 HMAC、ES256 和 RS256 的 JWT 验证。 |

本地运行时先按上面步骤安装 Chromium。Go 与浏览器测试会使用独立的 Compose 项目和卷，退出时自动清理。运行前可以停止手动启动的示例，或为测试设置另一组主机端口。

```sh
sh scripts/with-compose.sh sh -ec 'go test -tags=e2e -count=1 -v ./... && npm run test:browser'
```

Playwright 的 HTML 报告在 `playwright-report/index.html`，可打开各场景的 Trace 查看浏览器操作。只跑 Go E2E 可以直接运行 `sh scripts/with-compose.sh`。

## HTTP API 与配置

示例只启动 Kratos HTTP 服务。登录后的 API 请求使用 `Authorization: Bearer <token>`。

| 请求 | 用途 |
| --- | --- |
| `POST /demo/session` | 获取本地测试身份，Compose 默认启用。 |
| `POST /v1/upload/presign` | 提交 `filename`、`contentType` 和 `size`，返回对象键及签名 PUT 请求。支持 1 字节至 10 MiB 的文件。 |
| `POST /v1/upload/complete` | 提交对象 `key`，在请求内查询元数据并持久化确认结果。 |
| `GET /v1/upload` | 列出当前用户已验证的文件。 |
| `GET /v1/upload/{key}/download` | 为当前用户的已验证文件签发 GET 请求。 |

每个签名请求包含 `url`、`method`、`headers` 和 `expiresAt`，有效期五分钟。客户端使用原始签名 URL，并发送返回的请求头。确认前如果对象尚未写入，会返回 HTTP 409；存储临时失败时返回 HTTP 503，两种情况都可以重试。大小不符会持久化拒绝结果并返回 HTTP 422。

[Compose 配置](compose.yaml)展示了本地依赖、JWT 和存储地址的设置。服务端用 `UPLOAD_S3_ENDPOINT=http://minio:9000` 查对象元数据，签名 URL 使用 `UPLOAD_S3_PUBLIC_ENDPOINT` 对应的浏览器地址。

直连其他对象存储时，可设置 `UPLOAD_S3_ENDPOINT`、`UPLOAD_S3_REGION`、`UPLOAD_S3_BUCKET`、`UPLOAD_S3_PATH_STYLE` 和相应凭据。R2 使用 `https://<ACCOUNT_ID>.r2.cloudflarestorage.com` 和区域 `auto`。浏览器跨域请求需要在桶上配置页面来源、PUT/GET 和签名请求头的 CORS 规则。详细用法见 [storage/s3](../../storage/s3/README.md)。

使用真实 Supabase JWT 时，关闭 `UPLOAD_DEMO_LOGIN`，设置 `UPLOAD_JWT_ALGORITHM`、`UPLOAD_JWT_ISSUER` 和 `UPLOAD_JWT_AUDIENCE`。HMAC 模式配置 `UPLOAD_JWT_SECRET`；ES256 或 RS256 自动从 issuer 的 JWKS 地址获取公钥，也可通过 `UPLOAD_JWKS_URL` 指定地址。本地页面的登录按钮使用测试会话接口。日志默认写 stdout，设置 `UPLOAD_OTLP_LOG_ENDPOINT` 可同时发送 OTLP 日志。

## 构建与重新生成

应用所需的生成文件已经提交。在 `examples/upload` 用 Go 1.25.3 构建和测试。

```sh
GOWORK=off go build ./...
GOWORK=off go test ./...
```

修改 [upload.proto](api/upload/v1/upload.proto) 后，需要用 protoc 29.3 重新生成。见 [protoc 29.3 下载](https://github.com/protocolbuffers/protobuf/releases/tag/v29.3)。

```sh
sh scripts/generate.sh
```

脚本固定 Go protobuf 和 Kratos HTTP 生成器版本。可通过 `PROTOC` 指定编译器位置。CI 连续运行两次并检查生成目录的 Git 差异。
