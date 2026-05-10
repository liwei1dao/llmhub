# internal/vendors/

> 每家上游厂商的"私有适配器代码"放在这里。和 `internal/catalog/` 的关系：
> `catalog/` 只放**纯 metadata**（vendor / product / capability 字典 + 校验），
> `vendor/<id>/` 放**与该厂商绑定的可执行代码**（billing fetcher、capability adapter、
> 厂商专属的鉴权工具、错误码映射等）。

## 目录约定

```
internal/vendors/
├── README.md
├── all.go               (将来：聚合 RegisterAll(...) 把每家厂商挂到 runtime registry)
└── <vendor_id>/         (一家厂商一个子目录，目录名 = catalog.Vendors[id].ID)
    ├── billing/         (主账号余额/账单 fetcher。可选；当前阶段火山没接，所以是空的)
    ├── <board>/         (一个 VendorProduct 一个子目录，目录名 = product.ID 去掉 "<vendor>." 前缀)
    │   └── <capability>/  (一个 Capability 一个子目录)
    │       └── adapter.go  (实际调上游 API 的代码)
    └── vendor.go        (可选：厂商内部共用的工具 / 常量 / 错误码)
```

## 当前状态 (MVP)

只搭骨架，没有代码：

```
vendor/
├── README.md
└── volc/
    └── ark/
        └── chat/
            └── doc.go       (占位包；适配器接口定下来后再写)
```

LLMHub 是 **聚合 SDK 平台**，不代理上游请求 —— SDK 拿 lease 直连上游。所以
`vendor/<id>/<board>/<capability>/adapter.go` 这一类的"调用代码"短期内不会生长，
真正会先长起来的是：

- `vendor/<id>/billing/` —— 主账号余额查询、账单拉取（火山等接入了 OAuth/AK 之后）
- `vendor/<id>/auth_validator/` —— 创建 credential 时验证 key 是否真有效

## 加新厂商的步骤

1. 在 `internal/vendors/<new_id>/` 下建目录、写 `vendor.go`、`<board>/<capability>/`
2. 在 `internal/catalog/vendor.go` 的 `Vendors` map 加 metadata
3. 在 `internal/catalog/product.go` 的 `Products` map 加 board metadata
4. 在 `internal/catalog/capability.go` 的 `Capabilities` map 加 capability metadata（如属新增）
5. 跑 `go test ./internal/catalog/...` 校验 — 测试里的 `TestExpectedShape` 数字也要更新
6. 在 admin 前端 `accounts/_create-form.tsx` 的 `ENABLED_VENDORS` 白名单里把新 id 加进来
