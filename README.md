# LLMHub

> AI 能力聚合与分销平台 · 文本 / 语音 / 翻译 / 多模态

## 仓库结构

```
llmhub/
├── app/                  # 所有项目代码
│   ├── cmd/              # 6 个服务入口（gateway / scheduler / account / billing / worker / cli）
│   ├── internal/         # 业务代码（platform / domain / capability / provider / ...）
│   ├── pkg/              # 可对外复用的工具
│   ├── proto/            # gRPC proto + buf 配置
│   ├── migrations/       # goose SQL 迁移
│   ├── configs/          # 应用与 provider 配置（app.yaml、providers/*.yaml）
│   ├── deploy/           # docker-compose、Dockerfile、K8s manifest
│   ├── web/              # 3 个 Next.js 前端（www / console / admin）
│   ├── test/             # 协议一致性 golden + e2e
│   ├── tools/            # 构建期工具
│   ├── scripts/          # 运维脚本
│   ├── go.mod / go.sum
│   └── Makefile
│
├── docs/                 # 设计文档
├── preview/              # 产品预览静态页（单文件 HTML）
├── .github/workflows/    # CI（在 app/ 目录下执行 go/npm 命令）
├── Makefile              # 转发到 app/Makefile，可以从仓库根直接 make
└── README.md
```

## 快速开始

```bash
# 启动本地依赖（Postgres + Redis + NATS + Vault）
make dev-up

# 数据库迁移
make migrate-up

# 启动单个服务
make run-gateway
make run-scheduler

# 跑 Go 测试 + 强制的 golden 协议一致性测试
make test
make test-golden
```

所有 `make` 命令都通过根目录 Makefile 转发到 `app/Makefile`，也可以直接 `cd app && make ...`。

## 文档

- [业务需求](docs/01-业务需求文档.md)
- [项目框架](docs/02-项目框架设计.md)
- [数据结构](docs/03-核心数据结构设计.md)
- [接口设计](docs/04-模块接口设计.md)
- [能力域设计](docs/05-能力域专项设计.md)
- [Go 框架与扩展](docs/06-Go框架设计与厂商扩展.md)

## 服务列表

| 服务 | 端口 | 职责 |
|------|------|------|
| gateway | 8080 | 用户调用入口、协议转换、流式转发 |
| scheduler | 9001 | 账号池调度（gRPC） |
| account | 8081 / 9002 | 用户/钱包/Admin/UserAPI（HTTP + gRPC） |
| billing | 9003 | 计费、对账（gRPC） |
| worker | — | 后台任务：养号、对账、健康检查 |
| cli | — | 运维命令行 |

## 前端

| 应用 | 端口 | 用途 |
|------|------|------|
| app/web/www | 3000 | 官网/文档/博客（SSG，SEO 优先） |
| app/web/console | 3001 | 用户控制台（登录后使用） |
| app/web/admin | 3002 | 运营后台（登录后使用） |

## 许可

Proprietary · © 2026 LLMHub
