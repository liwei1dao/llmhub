# prod

生产环境 compose。**不暴露任何宿主机端口**——所有公网流量都由外层反向代理
（nginx / traefik / 云 LB）接管。外部依赖（postgres / redis / nats / vault）
由 `1panel-network` 上的运维实例提供。

prod compose 里 **不包含 migrate**：数据库迁移走单独的 CI/CD 流程，由发布
流水线在切流之前手动 / 自动执行，避免重启容器误触迁移。

## 服务访问

容器只用 `expose` 而不 `ports`，容器间在 `1panel-network` 上互通，公网走反代：

| 服务  | 容器内端口 | 反代后入口（按部署域名替换）        | 说明              |
| ----- | ---------- | ----------------------------------- | ----------------- |
| api   | 8080       | https://api.llmhub.io               | Go 单二进制       |
| web   | 3000       | https://www.llmhub.io               | Next.js 前台 / 控制台 |
| admin | 3001       | https://admin.llmhub.io             | Next.js 后台      |

> 实际域名以 `PUBLIC_API_BASE` 与反代配置为准。
> 健康检查：api 容器内置 `wget -qO- http://localhost:8080/health`，反代/LB
> 可以拿这条路径做存活探针。

## 默认超管账号

api 启动时若发现 `adminauth.admins` 表为空，会按 `LLMHUB_ADMIN_BOOTSTRAP_*`
环境变量自动种入第一个管理员；表非空时跳过，重启幂等。

prod **不内置默认账号**——`.env.example` 里三个 `LLMHUB_ADMIN_BOOTSTRAP_*`
全空，必须在 `.env.prod` 里手填首次发布用的强口令：

```env
LLMHUB_ADMIN_BOOTSTRAP_ACCOUNT=<企业邮箱 / 内部账号>
LLMHUB_ADMIN_BOOTSTRAP_PASSWORD=<≥16 位强口令，含大小写/数字/符号>
LLMHUB_ADMIN_BOOTSTRAP_NAME=<显示名>
```

**首次部署完成、确认能登录 admin 后**，立刻：

1. 清空 `.env.prod` 里的 `LLMHUB_ADMIN_BOOTSTRAP_PASSWORD`（或整组三个变量）；
2. `docker compose --env-file .env.prod up -d` 让 api 重新加载（表已非空，bootstrap 跳过）；
3. 在 admin 后台改/加正式的管理员账号，禁用 / 删除 bootstrap 账号。

## 启动 / 升级

```bash
# 通常路径：/opt/llmhub-prod
cp .env.example .env.prod      # 仔细填好所有密钥、tokens、bootstrap
docker login gitea.ideapsound.com
docker compose --env-file .env.prod pull
docker compose --env-file .env.prod up -d
```

升级版本：改 `.env.prod` 里 `IMAGE_TAG` 为固定 semver（**不要用 latest**），
先在维护窗口里手动跑 migrate（参考 dev compose 里的 migrate service 配置），
再 `pull && up -d`。

## 注意

- 不要把 `.env.prod` 提交进 git；建议放在部署机的 `/opt/llmhub-prod/`，权限 600。
- `LLMHUB_ADMIN_TOKEN` / `LLMHUB_INTERNAL_TOKEN` / `LLMHUB_VAULT_TOKEN` 一律
  从密钥管理系统注入，**不要复制 .env.example 里的占位串**。
