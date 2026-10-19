# aide —— 运行在你自己电脑上的 AI 助理

`aide` 是一个用 **Go** 编写、运行在本机的个人 AI 助理。它通过 **OpenAI 兼容接口**对接本地模型（默认 [Ollama](https://ollama.com)），数据不出本机；同时提供命令行（CLI）与一个本地 HTTP 服务（含内置网页）。

> 核心理念：**本地优先、最小权限、默认安全**。所有工具都是只读的，HTTP 服务默认只监听回环地址。

## 特性

- **多轮流式对话**：`aide chat` 进入 REPL，逐字输出，保留上下文。
- **一次性问答**：`aide ask "问题"`，支持管道 `echo ... | aide ask`。
- **代码解释**：`aide explain main.go:42`，用编码模型解释指定文件/行。
- **自动生成提交信息**：`aide commit` 读取已暂存改动，生成 Conventional Commits，默认只预览、`--apply` 才真正提交。
- **本地 HTTP 服务**：`aide serve` 提供 `POST /v1/chat` 与内置网页，默认 `127.0.0.1:8080`。
- **只读工具 + 路径安全**：`read_file` / `list_dir` 用 `EvalSymlinks` 校验，防止目录穿越与符号链接逃逸。
- **可观测、可测试**：`-race` 单测、集成测试、golangci-lint、govulncheck、GitHub Actions 全覆盖。

## 架构

```
cmd/aide/                 程序入口
internal/
  config/                 配置加载：默认值 < 配置文件 < 环境变量(AIDE_*)
  version/                版本信息（构建期 -ldflags 注入）
  llm/                    OpenAI 兼容客户端：Chat / ChatStream（SSE 解析）
  gitx/                   git 操作封装（diff/status/commit）
  agent/                  对话编排 + 工具调用循环 + 只读工具
  commands/               cobra 子命令：chat/ask/explain/commit/serve
  server/                 net/http 服务 + go:embed 网页
test/integration/         真实调用本地 Ollama 的集成测试
```

## 快速开始

### 前置

1. 安装并启动 [Ollama](https://ollama.com/download)，拉取一个模型：
   ```bash
   ollama pull qwen3.6:35b-mlx
   ```
2. 安装 Go（1.27+）。

### 构建与运行

```bash
make build          # 编译到 bin/aide
./bin/aide version
./bin/aide chat     # 开始对话
```

也可以直接 `make install` 后在任意目录使用 `aide`。

## 常用命令

```bash
aide chat                       # 多轮流式对话
aide ask "Go 里 context 的作用"  # 一次性问答
echo "解释这段代码" | aide ask   # 管道输入
aide explain main.go:42         # 解释文件第 42 行
aide commit                     # 预览提交信息（dry-run）
aide commit --apply             # 确认后真正提交
aide serve                      # 启动 HTTP 服务（含网页）
```

## 配置

加载优先级：**内置默认值 < 配置文件（config.yaml）< 环境变量（`AIDE_*`）**。

| 配置项 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| ServerURL | `AIDE_SERVER_URL` | `http://127.0.0.1:11434/v1` | OpenAI 兼容地址 |
| Model | `AIDE_MODEL` | `qwen3.6:35b-mlx` | 通用模型 |
| CoderModel | `AIDE_CODER_MODEL` | `qwen3-coder-next:latest` | 编码模型 |
| APIKey | `AIDE_API_KEY` | （空） | 仅云端时填写 |
| HTTPAddr | `AIDE_HTTP_ADDR` | `127.0.0.1:8080` | 服务监听地址 |
| WorkDir | `AIDE_WORK_DIR` | 当前目录 | 只读工具根目录 |
| Timeout | `AIDE_TIMEOUT` | `2m` | 非流式请求超时 |
| MaxFileBytes | `AIDE_MAX_FILE_BYTES` | `262144` | 单文件读取上限 |
| MaxToolTurns | `AIDE_MAX_TOOL_TURNS` | `6` | 工具调用最大轮数 |
| Temperature | `AIDE_TEMPERATURE` | `0.7` | 采样温度 |

参见 [.env.example](.env.example)。

## HTTP 服务

```bash
make serve
# 健康检查
curl http://127.0.0.1:8080/healthz
# 对话
curl -s http://127.0.0.1:8080/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"你好"}'
```

浏览器打开 <http://127.0.0.1:8080> 即可使用内置网页。

## 开发

```bash
make test           # 单元测试
make test-race      # 开启竞态检测
make cover          # 覆盖率报告
make lint           # golangci-lint（固定 v2.15.0）
make vet            # go vet
make dist           # 交叉编译 darwin/linux × arm64/amd64
make test-integration   # 真实调用本地 Ollama
```

## CI/CD

- **CI**（`.github/workflows/ci.yml`）：在 push / PR 时运行 golangci-lint、`-race` 测试（ubuntu/macos）、跨平台构建与 govulncheck。
- **Release**（`.github/workflows/release.yml`）：推送 `v*` tag 时由 [GoReleaser](https://goreleaser.com) 构建多平台二进制并生成校验和。

## 安全说明

- 本地模型优先，默认不依赖任何外部云服务。
- 工具仅能读取工作目录内文件，且全部为只读；通过 `EvalSymlinks` 阻止路径穿越与符号链接逃逸。
- HTTP 服务默认绑定 `127.0.0.1`，不暴露到局域网。
- 请勿将含真实密钥的 `config.yaml` / `.env` 提交到仓库（已在 `.gitignore` 中忽略）。

## License

[MIT](LICENSE)
