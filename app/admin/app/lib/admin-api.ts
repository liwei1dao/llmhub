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

export type AdminAccount = {
  id: number;
  provider_id: string;
  tier: string;
  origin: string;
  status: string;
  health_score: number;
  daily_used_cents: number;
  daily_limit_cents: number;
  qps_limit: number;
  supported_capabilities: string[] | null;
  tags: string[] | null;
  cost_basis_cents: number;
  quota_total_cents: number;
  quota_used_cents: number;
  api_key_ref: string;
  created_at: string;
};

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

export type ProviderRow = {
  id: string;
  display_name: string;
  status: string;
  protocol_family: string;
  base_url: string;
  capabilities: string[] | null;
};

export type PricingRow = {
  model_id: string;
  capability_id: string;
  unit: string;
  retail_cents_per_unit: number;
  cost_cents_per_unit: number;
  effective_at: string;
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

export type AccountEvent = {
  id: number;
  account_id: number;
  kind: string;
  reason: string | null;
  occurred_at: string;
};
