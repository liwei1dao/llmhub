// Server-side proxy that mirrors the Go SDK's chat flow so the
// "在线测试" panel in /services/[id] can demonstrate a real round-trip
// without hitting CORS at the upstream provider (Volc Ark / DeepSeek …).
//
// Flow:
//   1. POST {API_BASE}/sdk/credentials/issue  (Bearer user-api-key) → lease
//   2. POST {lease.endpoint}/chat/completions (Bearer auth_payload)  → response
//   3. POST {API_BASE}/sdk/usage/report       (Bearer user-api-key)  → ack
//
// The lease's auth_payload (the *real* upstream credential) never leaves
// the Next.js server process — only non-sensitive lease metadata is
// returned to the browser.

import { NextResponse } from 'next/server';

export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

const API_BASE = process.env.LLMHUB_API_BASE ?? process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:18080';

type ChatMessage = { role: string; content: string };

type TestRequest = {
  api_key: string;
  sku_id: string;
  messages: ChatMessage[];
  temperature?: number;
  max_tokens?: number;
  stream?: boolean;
};

type Lease = {
  lease_id: string;
  issued_at: string;
  expires_at: string;
  vendor: string;
  vendor_product: string;
  capability: string;
  upstream_model?: string;
  endpoint: string;
  protocol_family: string;
  auth_payload: Record<string, string>;
};

type UpstreamUsage = {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
};

// Mirror SDK's pickToken: Volc Ark accepts both `app_token` and `api_key`.
function pickToken(payload: Record<string, string>): string {
  for (const k of ['app_token', 'api_key']) {
    const v = (payload?.[k] ?? '').trim();
    if (v) return v;
  }
  return '';
}

function resolveUpstreamModel(lease: Lease): string {
  if (lease.upstream_model) return lease.upstream_model;
  return lease.auth_payload?.app_id ?? '';
}

async function issueLease(apiKey: string, skuId: string): Promise<Lease> {
  const r = await fetch(`${API_BASE}/sdk/credentials/issue`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${apiKey}`,
      'Cache-Control': 'no-store',
    },
    body: JSON.stringify({ sku_id: skuId }),
  });
  const text = await r.text();
  if (!r.ok) {
    throw new HTTPErr(r.status, text || `issue lease failed: ${r.status}`);
  }
  return JSON.parse(text) as Lease;
}

async function reportUsage(
  apiKey: string,
  leaseId: string,
  outcome: {
    input?: number;
    output?: number;
    latency_ms: number;
    ttfb_ms: number;
    status: 'success' | 'upstream_error' | 'rate_limited' | 'auth_failed' | 'timeout';
    error_code?: string;
  },
): Promise<void> {
  await fetch(`${API_BASE}/sdk/usage/report`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify({
      lease_id: leaseId,
      input_units: outcome.input ?? 0,
      output_units: outcome.output ?? 0,
      latency_ms: outcome.latency_ms,
      ttfb_ms: outcome.ttfb_ms,
      status: outcome.status,
      error_code: outcome.error_code ?? '',
    }),
  }).catch(() => {});
}

class HTTPErr extends Error {
  constructor(public status: number, msg: string) {
    super(msg);
  }
}

export async function POST(req: Request) {
  let body: TestRequest;
  try {
    body = (await req.json()) as TestRequest;
  } catch {
    return NextResponse.json({ error: { code: 'bad_request', message: 'invalid JSON' } }, { status: 400 });
  }

  if (!body.api_key || !body.api_key.trim()) {
    return NextResponse.json({ error: { code: 'missing_api_key', message: '请填写 API Key' } }, { status: 400 });
  }
  if (!body.sku_id) {
    return NextResponse.json({ error: { code: 'missing_sku', message: 'sku_id is required' } }, { status: 400 });
  }
  if (!Array.isArray(body.messages) || body.messages.length === 0) {
    return NextResponse.json({ error: { code: 'missing_messages', message: 'messages 不能为空' } }, { status: 400 });
  }

  let lease: Lease;
  try {
    lease = await issueLease(body.api_key.trim(), body.sku_id);
  } catch (e) {
    const err = e as HTTPErr;
    return NextResponse.json(
      { error: { code: 'issue_failed', message: err.message } },
      { status: err.status || 500 },
    );
  }

  const token = pickToken(lease.auth_payload);
  const upstreamModel = resolveUpstreamModel(lease);
  if (!token || !upstreamModel) {
    return NextResponse.json(
      { error: { code: 'bad_lease', message: 'lease 缺少 upstream_model 或 auth_payload bearer' } },
      { status: 502 },
    );
  }

  const upstreamBody: Record<string, unknown> = {
    model: upstreamModel,
    messages: body.messages,
  };
  if (body.temperature !== undefined) upstreamBody.temperature = body.temperature;
  if (body.max_tokens !== undefined) upstreamBody.max_tokens = body.max_tokens;

  const url = lease.endpoint.replace(/\/+$/, '') + '/chat/completions';
  const leaseMeta = {
    lease_id: lease.lease_id,
    vendor: lease.vendor,
    vendor_product: lease.vendor_product,
    capability: lease.capability,
    upstream_model: upstreamModel,
    endpoint: lease.endpoint,
    protocol_family: lease.protocol_family,
    issued_at: lease.issued_at,
    expires_at: lease.expires_at,
  };

  // ─── streaming branch ───
  if (body.stream) {
    upstreamBody.stream = true;
    const start = Date.now();
    const ttfbDeadline = { ms: 0 };

    let upstream: Response;
    try {
      upstream = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
          Accept: 'text/event-stream',
        },
        body: JSON.stringify(upstreamBody),
      });
    } catch (e) {
      const msg = (e as Error).message;
      await reportUsage(body.api_key.trim(), lease.lease_id, {
        latency_ms: Date.now() - start,
        ttfb_ms: 0,
        status: 'upstream_error',
        error_code: 'network_error',
      });
      return NextResponse.json({ error: { code: 'network_error', message: msg } }, { status: 502 });
    }
    ttfbDeadline.ms = Date.now() - start;

    if (!upstream.ok) {
      const errBody = await upstream.text();
      await reportUsage(body.api_key.trim(), lease.lease_id, {
        latency_ms: Date.now() - start,
        ttfb_ms: ttfbDeadline.ms,
        status: outcomeFromStatus(upstream.status),
        error_code: String(upstream.status),
      });
      return NextResponse.json(
        { error: { code: 'upstream_error', message: errBody.slice(0, 2000), status: upstream.status } },
        { status: 502 },
      );
    }

    // Pipe SSE back to the browser, then on close → report usage.
    const apiKey = body.api_key.trim();
    const encoder = new TextEncoder();
    const decoder = new TextDecoder();
    let lastUsage: UpstreamUsage | null = null;
    let buf = '';

    const out = new ReadableStream<Uint8Array>({
      async start(controller) {
        // Send lease meta first as a special SSE event so the UI can render it.
        controller.enqueue(encoder.encode(`event: lease\ndata: ${JSON.stringify(leaseMeta)}\n\n`));
        const reader = upstream.body!.getReader();
        try {
          for (;;) {
            const { done, value } = await reader.read();
            if (done) break;
            const chunk = decoder.decode(value, { stream: true });
            buf += chunk;
            // Sniff `usage` fields on each SSE event so we can report at the end.
            let nl: number;
            while ((nl = buf.indexOf('\n')) >= 0) {
              const line = buf.slice(0, nl);
              buf = buf.slice(nl + 1);
              const t = line.trim();
              if (t.startsWith('data:')) {
                const payload = t.slice(5).trim();
                if (payload && payload !== '[DONE]') {
                  try {
                    const frame = JSON.parse(payload) as { usage?: UpstreamUsage };
                    if (frame.usage) lastUsage = frame.usage;
                  } catch {
                    // ignore non-JSON SSE frames
                  }
                }
              }
            }
            controller.enqueue(value);
          }
        } catch (e) {
          controller.enqueue(
            encoder.encode(`event: error\ndata: ${JSON.stringify({ message: (e as Error).message })}\n\n`),
          );
        } finally {
          const latency = Date.now() - start;
          // Send a terminal summary event so the UI can render token usage.
          controller.enqueue(
            encoder.encode(
              `event: summary\ndata: ${JSON.stringify({
                usage: lastUsage,
                latency_ms: latency,
                ttfb_ms: ttfbDeadline.ms,
              })}\n\n`,
            ),
          );
          controller.close();
          await reportUsage(apiKey, lease.lease_id, {
            input: lastUsage?.prompt_tokens,
            output: lastUsage?.completion_tokens,
            latency_ms: latency,
            ttfb_ms: ttfbDeadline.ms,
            status: 'success',
          });
        }
      },
    });

    return new Response(out, {
      headers: {
        'Content-Type': 'text/event-stream; charset=utf-8',
        'Cache-Control': 'no-cache, no-transform',
        Connection: 'keep-alive',
      },
    });
  }

  // ─── non-streaming branch ───
  const start = Date.now();
  let upstream: Response;
  try {
    upstream = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
      body: JSON.stringify(upstreamBody),
    });
  } catch (e) {
    const msg = (e as Error).message;
    await reportUsage(body.api_key.trim(), lease.lease_id, {
      latency_ms: Date.now() - start,
      ttfb_ms: 0,
      status: 'upstream_error',
      error_code: 'network_error',
    });
    return NextResponse.json({ error: { code: 'network_error', message: msg } }, { status: 502 });
  }
  const ttfb = Date.now() - start;
  const text = await upstream.text();
  const latency = Date.now() - start;

  if (!upstream.ok) {
    await reportUsage(body.api_key.trim(), lease.lease_id, {
      latency_ms: latency,
      ttfb_ms: ttfb,
      status: outcomeFromStatus(upstream.status),
      error_code: String(upstream.status),
    });
    return NextResponse.json(
      {
        lease: leaseMeta,
        error: { code: 'upstream_error', status: upstream.status, message: text.slice(0, 4000) },
      },
      { status: 502 },
    );
  }

  let parsed: { usage?: UpstreamUsage; choices?: Array<{ message?: ChatMessage }> } & Record<string, unknown> = {};
  try {
    parsed = JSON.parse(text);
  } catch {
    return NextResponse.json(
      { lease: leaseMeta, error: { code: 'parse_error', message: 'upstream returned non-JSON', raw: text.slice(0, 2000) } },
      { status: 502 },
    );
  }

  await reportUsage(body.api_key.trim(), lease.lease_id, {
    input: parsed.usage?.prompt_tokens,
    output: parsed.usage?.completion_tokens,
    latency_ms: latency,
    ttfb_ms: ttfb,
    status: 'success',
  });

  return NextResponse.json({
    lease: leaseMeta,
    response: parsed,
    latency_ms: latency,
    ttfb_ms: ttfb,
  });
}

function outcomeFromStatus(status: number): 'upstream_error' | 'rate_limited' | 'auth_failed' | 'timeout' {
  if (status === 429) return 'rate_limited';
  if (status === 401 || status === 403) return 'auth_failed';
  if (status === 408 || status === 504) return 'timeout';
  return 'upstream_error';
}
