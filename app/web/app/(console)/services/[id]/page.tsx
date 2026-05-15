'use client';

import Link from 'next/link';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  api,
  API_BASE,
  type APIKey,
  type SDKRelease,
  type Subscription,
} from '@/lib/api';
import { fmtDateTime } from '@/lib/format';

// 已开通服务详情页。
//   Tab "overview"（默认）：服务说明 / 用量 / SDK 下载 / 文档
//   Tab "test"            ：网页测试面板（走 /api/sdk-test/chat 的 SSR 路由）
// Tab state 走 URL ?tab=test 以便分享 / 刷新保留位置。

type Tab = 'overview' | 'test';

export default function ServiceDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const search = useSearchParams();
  const skuId = decodeURIComponent(params?.id ?? '');
  const tab = (search.get('tab') === 'test' ? 'test' : 'overview') as Tab;

  const [subs, setSubs] = useState<Subscription[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: Subscription[] }>('/api/user/subscriptions')
      .then((r) => setSubs(r.data ?? []))
      .catch((e) => setError((e as Error).message));
  }, []);

  const sub = useMemo(() => subs?.find((s) => s.sku_id === skuId) ?? null, [subs, skuId]);

  function switchTab(next: Tab) {
    const url = next === 'overview' ? `/services/${encodeURIComponent(skuId)}` : `/services/${encodeURIComponent(skuId)}?tab=test`;
    router.push(url, { scroll: false });
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-xs text-ink-500">
        <Link href="/services" className="hover:text-ink-800">
          服务列表
        </Link>
        <span>/</span>
        <span className="mono text-ink-700">{skuId}</span>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800">{error}</div>
      ) : null}

      {subs === null ? (
        <div className="text-sm text-ink-500">加载中…</div>
      ) : !sub ? (
        <div className="rounded-2xl border border-dashed border-ink-300 bg-white px-5 py-12 text-center text-sm text-ink-500">
          没有找到这个订阅，可能尚未开通。
          <div className="mt-3">
            <Link href="/services" className="text-brand-700 underline">
              返回服务列表
            </Link>
          </div>
        </div>
      ) : (
        <>
          <ServiceHeader sub={sub} />
          <Tabs current={tab} onChange={switchTab} />
          {tab === 'overview' ? <OverviewPanel sub={sub} /> : <TestPanel sub={sub} />}
        </>
      )}
    </div>
  );
}

function ServiceHeader({ sub }: { sub: Subscription }) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-lg font-semibold text-ink-900">{sub.display_name ?? sub.sku_id}</div>
          <div className="mono mt-1 text-xs text-ink-500">{sub.sku_id}</div>
          <div className="mt-2 flex flex-wrap gap-2 text-[11px]">
            {sub.capability ? (
              <span className="rounded-full bg-brand-100 px-2 py-0.5 font-medium text-brand-700">
                {sub.capability}
              </span>
            ) : null}
            {sub.upstream_model ? (
              <span className="rounded-full bg-ink-200/60 px-2 py-0.5 text-ink-700">
                {sub.upstream_model}
              </span>
            ) : null}
            <span className="rounded-full bg-emerald-100 px-2 py-0.5 font-medium text-emerald-700">已开通</span>
          </div>
        </div>
        <QuotaBadge sub={sub} />
      </div>
    </div>
  );
}

function QuotaBadge({ sub }: { sub: Subscription }) {
  const pct = sub.quota_total > 0 ? Math.min(100, (sub.quota_used / sub.quota_total) * 100) : 0;
  const tone = pct >= 90 ? 'bg-rose-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500';
  return (
    <div className="min-w-[220px] flex-1 text-right">
      <div className="text-[11px] text-ink-500">本周期用量</div>
      <div className="mono mt-0.5 text-sm text-ink-800">
        {sub.quota_used.toLocaleString()} / {sub.quota_total.toLocaleString()} {sub.billing_unit ?? ''}
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-ink-200/60">
        <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function Tabs({ current, onChange }: { current: Tab; onChange: (t: Tab) => void }) {
  const items: { id: Tab; label: string }[] = [
    { id: 'overview', label: '服务详情' },
    { id: 'test', label: '在线测试' },
  ];
  return (
    <div className="flex gap-1 border-b border-ink-200">
      {items.map((it) => {
        const active = current === it.id;
        return (
          <button
            key={it.id}
            onClick={() => onChange(it.id)}
            className={`-mb-px border-b-2 px-4 py-2 text-sm transition ${
              active ? 'border-ink-900 font-medium text-ink-900' : 'border-transparent text-ink-500 hover:text-ink-800'
            }`}
          >
            {it.label}
          </button>
        );
      })}
    </div>
  );
}

// ─────────────────── Overview tab ───────────────────

function OverviewPanel({ sub }: { sub: Subscription }) {
  const [sdks, setSdks] = useState<SDKRelease[] | null>(null);
  const [sdkErr, setSdkErr] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: SDKRelease[] }>('/api/user/sdk/downloads')
      .then((r) => setSdks(r.data ?? []))
      .catch((e) => setSdkErr((e as Error).message));
  }, []);

  return (
    <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1fr_360px]">
      <div className="space-y-6">
        <SectionCard title="服务说明">
          <p className="text-sm leading-relaxed text-ink-700">
            该服务由 LLMHub 平台聚合，开发者通过下方 SDK 一键接入即可调用上游
            {sub.upstream_model ? <span className="mono"> {sub.upstream_model}</span> : null} 模型；
            平台只在 SDK 内分发短期 lease，不经手 prompt / 响应。
          </p>
          <dl className="mt-4 grid grid-cols-1 gap-y-2 text-sm md:grid-cols-2">
            <Row label="能力" value={sub.capability ?? '—'} />
            <Row label="计费单位" value={sub.billing_unit ?? '—'} />
            <Row label="QPS 上限" value={String(sub.qps_limit ?? '—')} />
            <Row label="计费方案" value={sub.plan_name || sub.plan_kind} />
            <Row label="周期开始" value={sub.period_start?.slice(0, 10) ?? '—'} />
            <Row label="周期结束" value={sub.period_end?.slice(0, 10) ?? '—'} />
            {sub.input_per_unit_cents !== undefined ? (
              <Row label="输入单价" value={`${sub.input_per_unit_cents.toFixed(2)} 分 / ${sub.billing_unit ?? ''}`} />
            ) : null}
            {sub.output_per_unit_cents !== undefined ? (
              <Row label="输出单价" value={`${sub.output_per_unit_cents.toFixed(2)} 分 / ${sub.billing_unit ?? ''}`} />
            ) : null}
          </dl>
        </SectionCard>

        <SectionCard title="SDK 使用情况">
          <div className="grid grid-cols-3 gap-4">
            <StatCell label="本周期已用" value={sub.quota_used.toLocaleString()} unit={sub.billing_unit} />
            <StatCell label="本周期剩余" value={sub.quota_remaining.toLocaleString()} unit={sub.billing_unit} />
            <StatCell label="今日已用" value={sub.daily_used.toLocaleString()} unit={sub.billing_unit} />
          </div>
          <div className="mt-4 flex items-center justify-between border-t border-ink-200 pt-3 text-xs text-ink-500">
            <span>每日上限：{sub.daily_quota_limit ? sub.daily_quota_limit.toLocaleString() : '无限制'}</span>
            <Link className="text-brand-700 hover:underline" href="/usage">
              查看详细用量曲线 →
            </Link>
          </div>
        </SectionCard>

        <SectionCard title="一行接入（Go）">
          <pre className="overflow-x-auto rounded-lg bg-ink-100/60 p-3 text-[11px] leading-relaxed">
{`c, _ := llmhub.New(llmhub.Config{ APIKey: "sk-llmh-..." })
resp, _ := c.Chat(ctx, &llmhub.ChatRequest{
  Model: "${sub.sku_id}",
  Messages: []llmhub.ChatMessage{
    {Role: "user", Content: "你好"},
  },
})`}
          </pre>
        </SectionCard>
      </div>

      <aside className="space-y-6">
        <SectionCard title="SDK 下载">
          {sdkErr ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700">{sdkErr}</div>
          ) : sdks === null ? (
            <div className="text-xs text-ink-500">加载中…</div>
          ) : sdks.length === 0 ? (
            <div className="text-xs text-ink-500">暂无 SDK 发布。</div>
          ) : (
            <ul className="space-y-3">
              {sdks.map((r) => (
                <li key={r.language} className="rounded-xl border border-ink-200 p-3">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-medium">{r.display_name}</div>
                    <SdkStatusPill status={r.status} />
                  </div>
                  <div className="mt-1 text-[11px] text-ink-500">
                    v{r.version} · {fmtDateTime(r.released_at)}
                  </div>
                  <pre className="mono mt-2 overflow-x-auto rounded bg-ink-100/60 px-2 py-1.5 text-[11px]">
                    {r.install_hint}
                  </pre>
                  <div className="mt-2 flex items-center gap-3 text-[11px]">
                    {r.download_url ? (
                      <a
                        className="text-brand-700 hover:underline"
                        href={r.download_url}
                        target="_blank"
                        rel="noreferrer"
                      >
                        下载
                      </a>
                    ) : null}
                    {r.docs_url ? (
                      <a
                        className="text-brand-700 hover:underline"
                        href={r.docs_url}
                        target="_blank"
                        rel="noreferrer"
                      >
                        文档
                      </a>
                    ) : null}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </SectionCard>

        <SectionCard title="相关文档">
          <ul className="space-y-2 text-sm">
            <DocLink href="/docs/sdk/go">Go SDK 接入文档</DocLink>
            <DocLink href="/docs/sdk/chat">Chat API 协议（OpenAI 兼容）</DocLink>
            <DocLink href="/docs/billing">计费与配额说明</DocLink>
            <DocLink href="/docs/errors">错误码速查</DocLink>
          </ul>
        </SectionCard>
      </aside>
    </div>
  );
}

function SectionCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="mb-3 text-sm font-semibold text-ink-900">{title}</div>
      {children}
    </section>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3 pr-4">
      <dt className="text-xs text-ink-500">{label}</dt>
      <dd className="text-right text-ink-800">{value}</dd>
    </div>
  );
}

function StatCell({ label, value, unit }: { label: string; value: string; unit?: string }) {
  return (
    <div className="rounded-xl border border-ink-200 bg-ink-100/40 px-3 py-2.5">
      <div className="text-[11px] text-ink-500">{label}</div>
      <div className="mono mt-1 text-lg font-semibold text-ink-900">
        {value}
        {unit ? <span className="ml-1 text-[11px] font-normal text-ink-500">{unit}</span> : null}
      </div>
    </div>
  );
}

function SdkStatusPill({ status }: { status: SDKRelease['status'] }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'beta'
        ? 'bg-amber-100 text-amber-700'
        : 'bg-ink-200 text-ink-600';
  return <span className={`rounded-full px-2 py-0.5 text-[10px] ${tone}`}>{status}</span>;
}

function DocLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <li>
      <Link className="flex items-center justify-between text-ink-700 hover:text-brand-700" href={href}>
        <span>{children}</span>
        <span className="text-ink-400">→</span>
      </Link>
    </li>
  );
}

// ─────────────────── Test tab ───────────────────

type ChatTurn = { role: 'user' | 'assistant'; content: string };

type LeaseMeta = {
  lease_id: string;
  vendor: string;
  vendor_product: string;
  capability: string;
  upstream_model: string;
  endpoint: string;
  protocol_family: string;
  issued_at: string;
  expires_at: string;
};

type ChatResp = {
  lease?: LeaseMeta;
  response?: {
    choices?: Array<{ message?: { role: string; content: string } }>;
    usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number };
  };
  latency_ms?: number;
  ttfb_ms?: number;
  error?: { code: string; message: string; status?: number };
};

function TestPanel({ sub }: { sub: Subscription }) {
  const onlyChat = sub.capability === 'chat' || sub.capability === undefined || sub.capability === null;

  // ─── api key picker ───
  // 选择 key 仅用于把 lease 归到这条 key 名下做用量归因。测试路径走
  // /api/user/sdk-test/chat（cookie 鉴权），不需要 key 的明文。
  const [keys, setKeys] = useState<APIKey[] | null>(null);
  const [pickedKeyID, setPickedKeyID] = useState<number | null>(null);
  const [keyError, setKeyError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: APIKey[] }>('/api/user/api-keys')
      .then((r) => {
        const list = (r.data ?? []).filter((k) => k.status === 'active');
        setKeys(list);
        setPickedKeyID(list[0]?.id ?? null);
      })
      .catch((e) => setKeyError((e as Error).message));
  }, []);

  const pickedKey = useMemo(() => keys?.find((k) => k.id === pickedKeyID) ?? null, [keys, pickedKeyID]);

  // ─── chat state ───
  const [system, setSystem] = useState('You are a helpful assistant.');
  const [input, setInput] = useState('用一句话介绍你自己。');
  const [temperature, setTemperature] = useState(0.7);
  const [maxTokens, setMaxTokens] = useState(512);
  const [stream, setStream] = useState(true);

  const [history, setHistory] = useState<ChatTurn[]>([]);
  const [streaming, setStreaming] = useState('');
  const [busy, setBusy] = useState(false);
  const [lastResp, setLastResp] = useState<ChatResp | null>(null);
  const [lastLease, setLastLease] = useState<LeaseMeta | null>(null);
  const [summary, setSummary] = useState<{
    usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number };
    latency_ms?: number;
    ttfb_ms?: number;
  } | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const send = useCallback(async () => {
    if (!pickedKey) {
      setErr('请先在「API Key」页面创建一个 key 用来归集这次测试的用量。');
      return;
    }
    if (!input.trim()) {
      setErr('请填写要发送的提问。');
      return;
    }
    setErr(null);
    setBusy(true);
    setStreaming('');
    setSummary(null);

    const userTurn: ChatTurn = { role: 'user', content: input };
    const messages = system.trim()
      ? [{ role: 'system', content: system.trim() }, ...history, userTurn]
      : [...history, userTurn];

    setHistory((h) => [...h, userTurn]);
    setInput('');

    const ac = new AbortController();
    abortRef.current = ac;

    const url = `${API_BASE}/api/user/sdk-test/chat`;
    const body = {
      sku_id: sub.sku_id,
      api_key_id: pickedKey.id,
      messages,
      temperature,
      max_tokens: maxTokens,
      stream,
    };

    try {
      const r = await fetch(url, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        signal: ac.signal,
      });

      if (stream) {
        if (!r.ok) {
          const j = await safeJSON(r);
          throw new Error(j?.error?.message ?? `${r.status} ${r.statusText}`);
        }
        let acc = '';
        await consumeSSE(r, {
          onLease: (l) => setLastLease(l),
          onDelta: (txt) => {
            acc += txt;
            setStreaming(acc);
          },
          onSummary: (s) => setSummary(s),
          onError: (m) => {
            throw new Error(m);
          },
        });
        if (acc) setHistory((h) => [...h, { role: 'assistant', content: acc }]);
        setStreaming('');
      } else {
        const j = await safeJSON(r);
        if (!r.ok || !j) {
          throw new Error(j?.error?.message ?? `${r.status} ${r.statusText}`);
        }
        setLastResp(j);
        if (j.lease) setLastLease(j.lease);
        const answer = j.response?.choices?.[0]?.message?.content ?? '';
        if (answer) {
          setHistory((h) => [...h, { role: 'assistant', content: answer }]);
        }
        setSummary({ usage: j.response?.usage, latency_ms: j.latency_ms, ttfb_ms: j.ttfb_ms });
      }
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
      abortRef.current = null;
    }
  }, [pickedKey, input, system, history, stream, temperature, maxTokens, sub.sku_id]);

  function cancel() {
    abortRef.current?.abort();
  }

  function clearChat() {
    setHistory([]);
    setStreaming('');
    setSummary(null);
    setLastResp(null);
  }

  if (!onlyChat) {
    return (
      <div className="rounded-2xl border border-dashed border-ink-300 bg-white px-5 py-12 text-center text-sm text-ink-500">
        这条 SKU 不是文本对话能力，请到 SDK 文档查看对应的调用方式。
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 gap-6 xl:grid-cols-[420px_1fr]">
      <aside className="space-y-4">
        <SectionCard title="API Key">
          {keys === null ? (
            <div className="text-xs text-ink-500">加载中…</div>
          ) : keys.length === 0 ? (
            <div className="text-xs text-ink-500">
              还没有可用的 key。先到{' '}
              <Link className="text-brand-700 hover:underline" href="/api-keys">
                API Key 页
              </Link>{' '}
              创建一个，回来再测。
            </div>
          ) : (
            <label className="block">
              <span className="mb-1 block text-[11px] text-ink-500">把这次测试归到哪条 key 名下</span>
              <select
                value={pickedKeyID ?? ''}
                onChange={(e) => setPickedKeyID(Number(e.target.value))}
                className="w-full rounded-lg border border-ink-300 bg-white px-3 py-2 text-sm focus:border-brand-500 focus:outline-none"
              >
                {keys.map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.name ?? `key-${k.id}`} · {k.prefix}…
                  </option>
                ))}
              </select>
              <span className="mt-2 block text-[11px] text-ink-500">
                这条路径走的是当前会话身份；选 key 只是为了把 lease / 用量记到这条 key 上，不需要 key 的明文。
              </span>
            </label>
          )}
          {keyError ? (
            <div className="mt-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700">{keyError}</div>
          ) : null}
        </SectionCard>

        <SectionCard title="参数">
          <div className="space-y-3">
            <NumberField label="temperature" value={temperature} step={0.1} min={0} max={2} onChange={setTemperature} />
            <NumberField label="max_tokens" value={maxTokens} step={64} min={1} max={8192} onChange={setMaxTokens} />
            <label className="flex items-center gap-2 text-xs text-ink-700">
              <input type="checkbox" checked={stream} onChange={(e) => setStream(e.target.checked)} />
              使用 SSE 流式
            </label>
            <div>
              <label className="mb-1 block text-[11px] text-ink-500">System Prompt</label>
              <textarea
                value={system}
                onChange={(e) => setSystem(e.target.value)}
                rows={2}
                className="w-full rounded-lg border border-ink-300 bg-white px-3 py-2 text-xs focus:border-brand-500 focus:outline-none"
              />
            </div>
          </div>
        </SectionCard>

        {lastLease ? <LeaseCard meta={lastLease} /> : null}
      </aside>

      <section className="space-y-4">
        <SectionCard title="对话">
          {history.length === 0 && !streaming ? (
            <div className="rounded-xl border border-dashed border-ink-300 bg-ink-100/30 px-4 py-10 text-center text-xs text-ink-500">
              还没有对话。在下方输入框开始一次真实的 SDK 调用。
            </div>
          ) : (
            <div className="space-y-3">
              {history.map((t, i) => (
                <Bubble key={i} turn={t} />
              ))}
              {streaming ? <Bubble turn={{ role: 'assistant', content: streaming }} streaming /> : null}
            </div>
          )}
          {summary ? (
            <div className="mt-3 border-t border-ink-200 pt-2 text-[11px] text-ink-500">
              {summary.usage
                ? `tokens: prompt ${summary.usage.prompt_tokens ?? '—'} · completion ${summary.usage.completion_tokens ?? '—'} · total ${summary.usage.total_tokens ?? '—'}`
                : null}
              {summary.latency_ms != null
                ? ` · 总耗时 ${summary.latency_ms}ms${summary.ttfb_ms ? ` · TTFB ${summary.ttfb_ms}ms` : ''}`
                : null}
            </div>
          ) : null}
        </SectionCard>

        <SectionCard title="输入">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            rows={4}
            placeholder="用户提问 …"
            className="w-full rounded-lg border border-ink-300 bg-white px-3 py-2 text-sm focus:border-brand-500 focus:outline-none"
          />
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
            <div className="text-[11px] text-ink-500">
              已积累 {history.length} 条历史消息。{history.length > 0 ? '后续提问会带上历史上下文。' : ''}
            </div>
            <div className="flex gap-2">
              <button
                onClick={clearChat}
                disabled={busy || history.length === 0}
                className="rounded-lg border border-ink-300 px-3 py-1.5 text-xs hover:bg-ink-200/40 disabled:opacity-50"
              >
                清空对话
              </button>
              {busy ? (
                <button
                  onClick={cancel}
                  className="rounded-lg border border-rose-400 px-3 py-1.5 text-xs text-rose-700 hover:bg-rose-50"
                >
                  取消
                </button>
              ) : (
                <button
                  onClick={send}
                  className="rounded-lg bg-ink-900 px-4 py-1.5 text-xs font-medium text-white hover:bg-ink-800"
                >
                  发送
                </button>
              )}
            </div>
          </div>
          {err ? (
            <div className="mt-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800">{err}</div>
          ) : null}
        </SectionCard>

        {!stream && lastResp ? <RawResponseCard data={lastResp} /> : null}
      </section>
    </div>
  );
}

function NumberField({
  label,
  value,
  step,
  min,
  max,
  onChange,
}: {
  label: string;
  value: number;
  step: number;
  min: number;
  max: number;
  onChange: (v: number) => void;
}) {
  return (
    <label className="flex items-center justify-between gap-3">
      <span className="text-[11px] text-ink-500">{label}</span>
      <input
        type="number"
        value={value}
        step={step}
        min={min}
        max={max}
        onChange={(e) => onChange(Number(e.target.value))}
        className="mono w-32 rounded-lg border border-ink-300 bg-white px-2 py-1 text-xs focus:border-brand-500 focus:outline-none"
      />
    </label>
  );
}

function Bubble({ turn, streaming }: { turn: ChatTurn; streaming?: boolean }) {
  const mine = turn.role === 'user';
  return (
    <div className={`flex ${mine ? 'justify-end' : 'justify-start'}`}>
      <div
        className={`max-w-[80%] whitespace-pre-wrap rounded-2xl px-3 py-2 text-xs ${
          mine ? 'bg-ink-900 text-white' : 'bg-ink-100 text-ink-800'
        }`}
      >
        {turn.content || (streaming ? '…' : '')}
        {streaming ? <span className="ml-1 animate-pulse">▍</span> : null}
      </div>
    </div>
  );
}

function LeaseCard({ meta }: { meta: LeaseMeta }) {
  return (
    <SectionCard title="最近一次 lease">
      <dl className="grid grid-cols-1 gap-y-1 text-xs">
        <Row label="lease_id" value={meta.lease_id} />
        <Row label="vendor" value={`${meta.vendor} / ${meta.vendor_product}`} />
        <Row label="protocol" value={meta.protocol_family} />
        <Row label="upstream_model" value={meta.upstream_model} />
        <Row label="endpoint" value={meta.endpoint} />
        <Row label="expires_at" value={meta.expires_at} />
      </dl>
      <div className="mt-2 text-[11px] text-ink-500">
        真实上游凭据（auth_payload）只在我们的 SSR 进程里出现，没有回到浏览器。
      </div>
    </SectionCard>
  );
}

function RawResponseCard({ data }: { data: ChatResp }) {
  return (
    <details className="rounded-2xl border border-ink-200 bg-white p-5 text-xs">
      <summary className="cursor-pointer text-sm font-semibold">原始响应 JSON</summary>
      <pre className="mt-3 max-h-80 overflow-auto rounded-lg bg-ink-100/60 p-3 text-[11px] leading-relaxed">
        {JSON.stringify(data.response, null, 2)}
      </pre>
    </details>
  );
}

async function safeJSON<T = ChatResp>(r: Response): Promise<T | null> {
  try {
    return (await r.json()) as T;
  } catch {
    return null;
  }
}

async function consumeSSE(
  r: Response,
  cb: {
    onLease: (m: LeaseMeta) => void;
    onDelta: (text: string) => void;
    onSummary: (s: {
      usage?: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number };
      latency_ms?: number;
      ttfb_ms?: number;
    }) => void;
    onError: (msg: string) => void;
  },
): Promise<void> {
  const reader = r.body!.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  let curEvent = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });

    let nl: number;
    while ((nl = buf.indexOf('\n')) >= 0) {
      const line = buf.slice(0, nl).replace(/\r$/, '');
      buf = buf.slice(nl + 1);

      if (line === '') {
        curEvent = '';
        continue;
      }
      if (line.startsWith('event:')) {
        curEvent = line.slice(6).trim();
        continue;
      }
      if (!line.startsWith('data:')) continue;
      const payload = line.slice(5).trim();
      if (!payload) continue;

      if (curEvent === 'lease') {
        try { cb.onLease(JSON.parse(payload) as LeaseMeta); } catch { /* ignore */ }
        continue;
      }
      if (curEvent === 'summary') {
        try { cb.onSummary(JSON.parse(payload)); } catch { /* ignore */ }
        continue;
      }
      if (curEvent === 'error') {
        try {
          const e = JSON.parse(payload) as { message?: string };
          cb.onError(e.message ?? 'stream error');
        } catch {
          cb.onError('stream error');
        }
        continue;
      }
      if (payload === '[DONE]') continue;
      try {
        const frame = JSON.parse(payload) as {
          choices?: Array<{ delta?: { content?: string } }>;
        };
        const delta = frame.choices?.[0]?.delta?.content;
        if (delta) cb.onDelta(delta);
      } catch {
        // skip non-JSON keep-alive frames
      }
    }
  }
}
