# deploy

仓库顶层部署目录。覆盖 LLMHub 的三个业务镜像：

- `llmhub-api`  : Go 单二进制（`app/services`）
- `llmhub-web`  : Next.js 前台 / 控制台（`app/web`，3000）
- `llmhub-admin`: Next.js 后台（`app/admin`，3001）

外部依赖（Postgres / Redis / NATS / Vault）一律不在 compose 内拉起，
**全部通过环境变量注入连接信息**，由宿主机或外层运维体系提供。
所有容器都挂在外部 Docker 网络 `1panel-network` 上：

```yaml
networks:
  1panel-network:
    external: true
```

## 目录结构

```
deploy/
├── README.md
├── local/    # 本机开发: 复用宿主机上已经运行的 pg / redis / nats / vault
│   ├── .env.example
│   └── docker-compose.yml
├── dev/      # 开发/测试: 拉远端镜像，连远端运维实例
│   ├── .env.example
│   └── docker-compose.yml
└── prod/     # 生产: 不暴露内部端口，由外层反代接管
    ├── .env.example
    └── docker-compose.yml
```

## 镜像构建

```bash
# 在仓库根目录执行
docker build -t llmhub-api:local   -f app/services/deploy/Dockerfile app/services
docker build -t llmhub-web:local   -f app/web/Dockerfile             app/web
docker build -t llmhub-admin:local -f app/admin/Dockerfile           app/admin
```

## 启动

```bash
cd deploy/local
cp .env.example .env.local        # 改成你自己的连接串
docker compose --env-file .env.local up -d
```
