# local

本机开发用 compose。**只拉起 llmhub 自己的三个服务 + 一次性的 migrate**；
postgres / redis / nats / vault 复用宿主机上已经在跑的实例，通过外部网络
`1panel-network` 直连。

## 服务访问

| 服务  | 宿主机端口 | URL                       | 说明                |
| ----- | ---------- | ------------------------- | ------------------- |
| api   | **18080**  | http://localhost:18080    | Go 单二进制         |
| web   | 13000      | http://localhost:13000    | Next.js 前台 / 控制台 |
| admin | 13001      | http://localhost:13001    | Next.js 后台        |

> api 默认改用 **18080** 而非 8080，是为了避开 lottery / 其他本地项目占用的 8080。
> 健康检查：`curl http://localhost:18080/health`。

## 默认超管账号

api 启动时若发现 `adminauth.admins` 表为空，会按 `LLMHUB_ADMIN_BOOTSTRAP_*`
环境变量自动种入第一个管理员；表非空时跳过，所以**重启幂等**。

`.env.example` 里的默认值：

| 字段     | 值            |
| -------- | ------------- |
| 账号     | `admin`       |
| 密码     | `admin123`    |
| 显示名   | `Local Admin` |
| 登录入口 | http://localhost:13001 |

如果之前已经手动建过管理员，自动注入会被跳过；想强制重置：在 postgres 里
`TRUNCATE adminauth.admins`，然后重启 api。

## 启动 / 重启

镜像构建（仓库根目录执行）：

```bash
docker build -t llmhub-api:local   -f app/services/deploy/Dockerfile app/services
docker build --build-arg NEXT_PUBLIC_API_BASE=http://localhost:18080 \
             -t llmhub-web:local   -f app/web/Dockerfile             app/web
docker build --build-arg NEXT_PUBLIC_API_BASE=http://localhost:18080 \
             -t llmhub-admin:local -f app/admin/Dockerfile           app/admin
```

启动：

```bash
cd deploy/local
cp .env.example .env       # 第一次：拷模板填实际值
docker compose --env-file .env up -d
```

查看日志：

```bash
docker compose --env-file .env logs -f api
```

## 注意

- 宿主机 postgres 当前 superuser 是 `tradestock`，`llmhub` 库的 owner 也是它。
  如果换了 superuser，记得同步改 `.env` 的 `LLMHUB_DB_DSN`。
- migrate 是一次性 job，正常会 exit 0；api 用 `depends_on: service_completed_successfully`
  等它结束才启动。
