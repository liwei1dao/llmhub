// Thin fetch wrapper for the account service. The account service hosts
// /api/user/* and /api/admin/* on the same binary; the console only
// touches /api/user/*.
//
// Cookies are forwarded automatically (`credentials: include`) so the
// session set by /auth/login persists across pages.

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8081';

export type APIError = { code: string; message: string };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    cache: 'no-store',
  });
  if (!res.ok) {
    const text = await res.text();
    let parsed: APIError | null = null;
    try {
      parsed = JSON.parse(text) as APIError;
    } catch {
      // not JSON — fall through to raw text
    }
    throw new Error(parsed?.message ?? `${res.status} ${res.statusText}: ${text}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T,>(p: string) => call<T>('GET', p),
  post: <T,>(p: string, body?: unknown) => call<T>('POST', p, body),
};

// ---- types matching the Go handlers ----

export type Profile = {
  id: number;
  email: string | null;
  phone: string | null;
  status: string;
  qps_limit: number;
};

export type Wallet = {
  balance_cents: number;
  frozen_cents: number;
  currency: string;
  total_recharged_cents: number;
  total_spent_cents: number;
};

export type APIKey = {
  id: number;
  prefix: string;
  name: string | null;
  scopes: string[] | null;
  status: string;
  created: string;
};

export type CreatedAPIKey = { id: number; prefix: string; key: string };

export type UsageBucket = {
  day: string;
  capability_id: string;
  calls: number;
  success_calls: number;
  tokens_in: number;
  tokens_out: number;
  audio_seconds: number;
  characters: number;
  cost_retail_cents: number;
};

export type Recharge = {
  order_no: string;
  amount_cents: number;
  channel: string;
  status: string;
  created_at: string;
};
