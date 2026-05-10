// Admin API client. Talks to the same account-service binary that hosts
// /api/admin/* routes guarded by the X-Admin-Token shared secret.
//
// The token is held in localStorage on the operator's machine. In M9
// this becomes a session-cookie + RBAC scheme.

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8081';
const TOKEN_KEY = 'llmhub_admin_token';

export function getToken(): string {
  if (typeof window === 'undefined') return '';
  return window.localStorage.getItem(TOKEN_KEY) ?? '';
}
export function setToken(t: string) {
  if (typeof window === 'undefined') return;
  if (t) window.localStorage.setItem(TOKEN_KEY, t);
  else window.localStorage.removeItem(TOKEN_KEY);
}

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = getToken();
  if (!token) throw new Error('未登录：请先在登录页输入管理员 token');
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: {
      'X-Admin-Token': token,
      ...(body ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
    cache: 'no-store',
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${text}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T,>(p: string) => call<T>('GET', p),
  post: <T,>(p: string, body?: unknown) => call<T>('POST', p, body),
  patch: <T,>(p: string, body?: unknown) => call<T>('PATCH', p, body),
  delete: <T,>(p: string) => call<T>('DELETE', p),
};

// ---- shared types ----
//
// v0.1 的 AdminAccount / ProviderRow / PricingRow 已随中间站代码一起移除。
// 现在的 admin 模型见下面 v0.2 五层 (Vendor / VendorProduct / VendorAccount /
// Credential / ServiceBinding) + PlatformService SKU。

export type AdminUser = {
  id: number;
  email: string | null;
  phone: string | null;
  status: string;
  risk_score: number;
  qps_limit: number;
  daily_spend_limit_cents: number;
  created_at: string;
  last_login_at: string | null;
};

export type ReconRow = {
  day: string;
  provider_id: string;
  account_id: number;
  upstream_cents: number;
  metered_cents: number;
  delta_cents: number;
  status: string;
};


// ────────────────────────────────────────────────────────────
// v0.2 五层模型：catalog 字典 + pool 资源
// ────────────────────────────────────────────────────────────

export type FieldSpec = {
  key: string;
  label: string;
  sensitive?: boolean;
  required?: boolean;
  pattern?: string;
};

export type PlatformCategory = {
  id: string;
  name: string;
  sort_order: number;
};

export type Vendor = {
  id: string;
  name: string;
  logo_url?: string;
  console_url?: string;
  master_auth_schema: FieldSpec[];
  // 服务端把 products 嵌进来（见 admin.listVendors）
  products?: VendorProduct[];
};

export type VendorProduct = {
  id: string;
  vendor_id: string;
  name: string;
  credential_schema: FieldSpec[];
  allowed_capabilities: string[];
  protocol_family: string;
  endpoint_template?: string;
};

export type Capability = {
  id: string;
  category_id: string;
  display_name: string;
  billing_unit: string;
};

export type VendorAccount = {
  id: number;
  vendor_id: string;
  name: string;
  entity?: string;
  console_url?: string;
  status: string;
  last_balance_cents?: number;
  last_balance_currency?: string;
  last_balance_at?: string;
  last_balance_error?: string;
  created_at: string;
  updated_at: string;
};

export type Credential = {
  id: number;
  vendor_id: string;
  account_id: number;
  product_id: string;
  name: string;
  env: string;
  status: string;
  health_score: number;
  isolation_group_id?: number | null;
  consecutive_failures: number;
  cooldown_until?: string;
  last_used_at?: string;
  last_error_at?: string;
  created_at: string;
  updated_at: string;
};

export type ServiceBinding = {
  id: number;
  credential_id: number;
  capability: string;
  tier: string;
  qps_limit?: number | null;
  daily_limit_cents?: number | null;
  quota_total_cents?: number | null;
  daily_used_cents: number;
  cost_basis_cents: number;
  health_score: number;
  status: string;
};

export type CredentialDetail = {
  credential: Credential;
  bindings: ServiceBinding[];
};

// 创建凭据时一次提交：1 凭据 + N binding 同事务
export type CreateCredentialReq = {
  account_id: number;
  product_id: string;
  name: string;
  env?: string;
  isolation_group_id?: number | null;
  auth_payload: Record<string, string>;
  bindings: {
    capability: string;
    tier: string;
    qps_limit?: number | null;
    daily_limit_cents?: number | null;
    quota_total_cents?: number | null;
    cost_basis_cents: number;
  }[];
};

// catalog.platform_services + 内联当前 retail 价
export type PlatformService = {
  id: string;
  category_id: string;
  display_name: string;
  description?: string;
  vendor_product_id: string;
  capability: string;
  upstream_model?: string;
  billing_unit: string;
  context_window?: number;
  max_output_tokens?: number;
  is_public: boolean;
  sort_order: number;
  tags?: string[];
  status: string;
  created_at: string;
  updated_at: string;
  current_input_cents?: number;
  current_output_cents?: number;
  current_image_cents?: number;
};

export type CreatePlatformServiceReq = {
  id: string;
  category_id: string;
  display_name: string;
  description?: string;
  vendor_product_id: string;
  capability: string;
  upstream_model?: string;
  billing_unit: string;
  context_window?: number;
  max_output_tokens?: number;
  is_public?: boolean;
  sort_order?: number;
  tags?: string[];
  input_per_unit_cents?: number;
  output_per_unit_cents?: number;
  image_per_unit_cents?: number;
};

// SKU 改价：追加新 effective_from=NOW 的 pricing 行，旧的 effective_until 自动收尾
export type UpdatePricingReq = {
  input_per_unit_cents?: number;
  output_per_unit_cents?: number;
  image_per_unit_cents?: number;
  notes?: string;
};

// ───────────────────────── 用户订阅 ─────────────────────────

export type AdminSubscription = {
  id: number;
  user_id: number;
  sku_id: string;
  sku_name?: string;
  plan_kind: string;
  plan_name?: string;
  quota_total: number;
  quota_used: number;
  quota_remaining: number;
  period_start: string;
  period_end: string;
  auto_renew: boolean;
  status: string;
  qps_limit: number;
  daily_quota_limit?: number;
  daily_used: number;
  created_at: string;
  updated_at: string;
};

export type GrantSubscriptionReq = {
  sku_id: string;
  plan_kind: 'monthly' | 'prepaid' | 'trial';
  plan_name?: string;
  quota_total: number;
  period_end?: string;            // RFC3339; default 30d / 90d / 7d by plan_kind
  auto_renew?: boolean;
  qps_limit?: number;
  daily_quota_limit?: number;
};

export type PatchSubscriptionReq = {
  quota_total?: number;
  period_end?: string;
  auto_renew?: boolean;
  qps_limit?: number;
  daily_quota_limit?: number;
  status?: 'active' | 'suspended' | 'cancelled' | 'expired';
  plan_name?: string;
};

// ───────────────────────── 活 lease 监控 ─────────────────────────

// pool.leases 行（admin 视角）。auth_payload_ref 字段后端不暴露，
// 这里也对应没有 — 管理员看不到 vault 路径。
export type AdminLease = {
  lease_id: string;
  user_id: number;
  api_key_id: number;
  sku_id: string;
  binding_id: number;
  credential_id: number;
  client_fingerprint?: string;
  client_ip?: string;
  status: string;
  issued_at: string;
  expires_at: string;
  revoked_at?: string;
  revoke_reason?: string;
  last_used_at?: string;
  use_count: number;
  total_input_units: number;
  total_output_units: number;
};

// ───────────────────────── 总览指标 ─────────────────────────

// /api/admin/dashboard/stats 返回的扁平计数。所有字段在某些资源
// 不存在时可能 missing — 前端按 ?? 0 兜底。
export type DashboardStats = {
  server_time?: string;
  leases_active?: number;
  leases_total?: number;
  subscriptions_active?: number;
  subscriptions_total?: number;
  users_total?: number;
  api_keys_active?: number;
  credentials_active?: number;
  credentials_cooldown?: number;
  calls_today?: number;
  calls_success_today?: number;
  recharges_pending?: number;
};

// ───────────────────────── 审计日志 ─────────────────────────

export type AuditLog = {
  id: number;
  actor_type: string;     // admin / user / system / sdk
  actor_id: number | null;
  action: string;         // grant_subscription / revoke_lease / update_pricing / ...
  target_type: string;    // user / subscription / lease / platform_service / ...
  target_id: string;
  ip: string;
  user_agent: string;
  result: string;         // ok / error / denied
  payload?: unknown;      // 解包过的 JSON：随 action 形状不同
  created_at: string;     // RFC3339 UTC
};

// 用户详情
export type AdminUserDetail = {
  id: number;
  email: string | null;
  phone: string | null;
  status: string;
  display_name?: string;
  realname_level?: number;
  risk_score: number;
  qps_limit: number;
  daily_spend_limit_cents: number;
  created_at: string;
  updated_at: string;
  last_login_at: string | null;
};

// 用户钱包快照（admin 专用）：余额 + 最近 10 条充值 + 30 日消费汇总
export type AdminUserWallet = {
  account_exists: boolean;
  balance_cents?: number;
  frozen_cents?: number;
  currency?: string;
  total_recharged_cents?: number;
  total_spent_cents?: number;
  spent_30d_cents?: number;
  calls_30d?: number;
  recharges: AdminRecharge[];
};

export type AdminRecharge = {
  order_no: string;
  amount_cents: number;
  channel: string;             // alipay / wechat / stripe / manual
  status: string;              // pending / paid / failed / refunded / cancelled
  paid_at: string | null;
  created_at: string;
  channel_order_id: string | null;
};
