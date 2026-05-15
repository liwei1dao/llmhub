// Package admin exposes the operations REST surface: pool management,
// provider catalog mutations, user support. Hosted inside the account
// service binary.
package admin

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/llmhub/llmhub/internal/adminauth"
	"github.com/llmhub/llmhub/internal/audit"
	catalogrepo "github.com/llmhub/llmhub/internal/catalog/repo"
	iamrepo "github.com/llmhub/llmhub/internal/iam/repo"
	meteringrepo "github.com/llmhub/llmhub/internal/metering/repo"
	"github.com/llmhub/llmhub/internal/pool"
	poolrepo "github.com/llmhub/llmhub/internal/pool/repo"
	"github.com/llmhub/llmhub/internal/wallet"
)

// Server owns the admin router.
type Server struct {
	logger   *slog.Logger
	repo     *poolrepo.Repo
	pool     *pool.Service // v0.2 service: vendor accounts, credentials, bindings
	iam      *iamrepo.Repo
	catalog  *catalogrepo.Repo
	metering *meteringrepo.Repo
	wallet   *wallet.Service
	auth     *adminauth.Service // back-office login / sessions
	audit    audit.Recorder     // optional; defaults to audit.Nop{}
}

// WithPool plugs in the v0.2 pool service.
func (s *Server) WithPool(p *pool.Service) *Server { s.pool = p; return s }

// New builds a Server. The auth wiring is done via WithAuth — we no
// longer take a shared-secret token; admin login goes through
// account+password against the adminauth schema.
func New(logger *slog.Logger, repo *poolrepo.Repo) *Server {
	return &Server{logger: logger, repo: repo, audit: audit.Nop{}}
}

// WithAuth plugs in the back-office identity service. Required for
// admin auth middleware and the /auth/* routes.
func (s *Server) WithAuth(a *adminauth.Service) *Server { s.auth = a; return s }

// WithAudit attaches an audit.Recorder so admin mutations get logged
// to audit.logs. Without this, operations still succeed but no record
// is kept — only acceptable in tests.
func (s *Server) WithAudit(r audit.Recorder) *Server { s.audit = r; return s }

// WithIAM plugs in the iam repo for user-admin endpoints.
func (s *Server) WithIAM(r *iamrepo.Repo) *Server { s.iam = r; return s }

// WithCatalog plugs in the catalog repo for pricing/provider views.
func (s *Server) WithCatalog(r *catalogrepo.Repo) *Server { s.catalog = r; return s }

// WithMetering plugs in the metering repo for reconciliation views.
func (s *Server) WithMetering(r *meteringrepo.Repo) *Server { s.metering = r; return s }

// WithWallet plugs in the wallet service for recharge confirmation.
func (s *Server) WithWallet(w *wallet.Service) *Server { s.wallet = w; return s }

// Mount registers /api/admin/* routes on r.
//
// 在「聚合 SDK 平台」模式下，admin 后台只剩 4 块：
//   - 静态目录字典（catalog dict）— vendors / products / capabilities / categories
//   - 上游凭据池（vendor accounts / credentials / bindings）
//   - 平台 SKU + 定价（platform services / pricing）
//   - 用户管理 + 充值确认 + 计量观察
//
// 老的 /pool/accounts、/providers、/pricing 端点跟着 v0.1 catalog/pool
// 表一起在 Phase 9 里删除了（中间站时代的产物）。
func (s *Server) Mount(r chi.Router) {
	r.Route("/api/admin", func(r chi.Router) {
		// 公开 auth 路由（无需登录态即可访问）
		r.Post("/auth/login", s.handleAdminLogin)

		// 受 admin 会话保护的路由
		r.Group(func(r chi.Router) {
			r.Use(s.requireAdmin)

			// 当前登录态
			r.Post("/auth/logout", s.handleAdminLogout)
			r.Get("/auth/me", s.handleAdminMe)

			// 静态目录字典（代码常量层）
			r.Route("/catalog", func(r chi.Router) {
				r.Get("/categories", s.listCategories)
				r.Get("/vendors", s.listVendors)
				r.Get("/products", s.listProducts)
				r.Get("/capabilities", s.listCapabilities)
			})

			// 服务模块（代码侧注册）— admin「服务列表」页用，
			// 列出可被上架的服务模块 + 各自开放的模型 / 节点参数空间。
			r.Get("/service-modules", s.listServiceModules)

			// 上游账号池（账户级别）
			r.Route("/vendor-accounts", func(r chi.Router) {
				r.Get("/", s.listVendorAccounts)
				r.Post("/", s.createVendorAccount)
				r.Get("/{id}", s.getVendorAccount)
				r.Patch("/{id}", s.patchVendorAccount)
				r.Delete("/{id}", s.archiveVendorAccount)
			})

			// 凭据 + 调度行（绑定）
			r.Route("/credentials", func(r chi.Router) {
				r.Get("/", s.listCredentials)
				r.Post("/", s.createCredential)
				r.Get("/{id}", s.getCredential)
				r.Delete("/{id}", s.archiveCredential)
				r.Patch("/{id}/auth-payload", s.rotateAuthPayload)
				r.Post("/{id}/bindings", s.addBinding)
				r.Get("/{id}/events", s.listCredentialEvents)
			})

			// 平台 SKU + 定价
			r.Route("/platform-services", func(r chi.Router) {
				r.Get("/", s.listPlatformServices)
				r.Post("/", s.createPlatformService)
				r.Patch("/{id}", s.patchPlatformService)
				r.Post("/{id}/pricing", s.updatePlatformServicePricing)
			})

			// 活 lease 监控 + 强制撤销
			r.Route("/leases", func(r chi.Router) {
				r.Get("/", s.listLeases)
				r.Delete("/{lease_id}", s.revokeLease)
			})

			// 用户订阅（管理员代客开订 / 调配额 / 取消）
			r.Route("/users/{id}/subscriptions", func(r chi.Router) {
				r.Get("/", s.listUserSubscriptions)
				r.Post("/", s.grantSubscription)
			})
			r.Route("/subscriptions", func(r chi.Router) {
				r.Patch("/{id}", s.patchSubscription)
				r.Delete("/{id}", s.cancelSubscription)
			})

			// 总览数据
			r.Get("/dashboard/stats", s.dashboardStats)

			// 审计日志
			r.Get("/audit-logs", s.auditList)

			// 用户 / 充值 / 对账
			r.Get("/users", s.listUsers)
			r.Get("/users/{id}", s.getUser)
			r.Get("/users/{id}/wallet", s.getUserWallet)
			r.Get("/users/{id}/usage", s.getUserUsage)
			r.Get("/reconciliation", s.listRecon)
			r.Post("/recharges/{order_no}/confirm", s.handleConfirmRecharge)
		})
	})
}
