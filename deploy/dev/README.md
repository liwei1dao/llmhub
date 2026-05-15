# dev

开发 / 测试环境。镜像从 `gitea.ideapsound.com` 拉取，外部依赖
（postgres / redis / nats / vault）由 `1panel-network` 上的运维实例提供，
compose **不自己拉起**。

## 服务访问

镜像本身仍监听容器内 8080 / 3000 / 3001，宿主机映射如下：

| 服务  | 宿主机端口 | URL（按部署机器替换 host）   | 说明                |
| ----- | ---------- | ---------------------------- | ------------------- |
| api   | 8080       | http://dev-host:8080         | Go 单二进制         |
| web   | 3000       | http://dev-host:3000         | Next.js 前台        |
| admin | 3001       | http://dev-host:3001         | Next.js 后台        |

> 部署机器的实际 host / 域名以运维约定为准；如果外面挂了反代，访问走反代地址。
> 健康检查：`curl http://dev-host:8080/health`。

## 默认超管账号

api 启动时若发现 `adminauth.admins` 表为空，会按 `LLMHUB_ADMIN_BOOTSTRAP_*`
环境变量自动种入第一个管理员；表非空时跳过，重启幂等。

dev 环境**没有内置默认值**——`.env.example` 三个 `LLMHUB_ADMIN_BOOTSTRAP_*`
字段都是空的，需要在 `.env.dev` 里填好首次部署用的账号：

```env
LLMHUB_ADMIN_BOOTSTRAP_ACCOUNT=dev_admin
LLMHUB_ADMIN_BOOTSTRAP_PASSWORD=<强口令>
LLMHUB_ADMIN_BOOTSTRAP_NAME=Dev Admin
```

登录入口：http://dev-host:3001。

首次启动后建议把 `LLMHUB_ADMIN_BOOTSTRAP_PASSWORD` 从 `.env.dev` 清空 —— 反正
表非空就不会重新注入，留着只是泄露风险。

## 启动 / 升级

```bash
cd deploy/dev
cp .env.example .env.dev   # 填好 USER/PASS、tokens、bootstrap 账号
docker login gitea.ideapsound.com
docker compose --env-file .env.dev pull
docker compose --env-file .env.dev up -d
```

切版本：改 `.env.dev` 里的 `IMAGE_TAG`，再 `pull && up -d`。

## 注意

- `migrate` 是一次性 job，正常 exit 0；api 用 `depends_on:
  service_completed_successfully` 等它结束才启动。迁移失败时 api 不会被拉起。
- `LLMHUB_DB_DSN` 用的 host 是容器名（如 `postgres`），不是宿主机 IP，因为
  所有服务都挂在 `1panel-network` 上。
