'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import {
  api,
  type CatalogService,
  type ServiceCatalog,
  type Subscription,
} from '@/lib/api';

// 服务列表 = 用户视角的"服务中心"。
//   - 主区：已开通服务（来自 /api/user/subscriptions）
//   - "开通服务" 按钮：弹出可开通服务目录（/api/user/services/catalog）
// MVP 阶段，目录里只展示「大模型 → 文本对话」分支下火山的 SKU。其它
// 大类（语音/翻译/视觉）做"敬请期待"的灰态占位，避免用户误以为可用。
export default function ServicesPage() {
  const [subs, setSubs] = useState<Subscription[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);

  useEffect(() => {
    api
      .get<{ data: Subscription[] }>('/api/user/subscriptions')
      .then((r) => setSubs(r.data ?? []))
      .catch((e) => setError((e as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">服务列表</h1>
          <p className="mt-1 text-sm text-ink-500">
            已开通的平台服务一览。点击右上角「开通服务」选择更多 SKU 接入；当前阶段仅开放大模型 · 文本对话。
          </p>
        </div>
        <button
          onClick={() => setPickerOpen(true)}
          className="shrink-0 rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white hover:bg-ink-800"
        >
          开通服务
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800">
          {error}
        </div>
      ) : null}

      {subs === null ? (
        <div className="text-sm text-ink-500">加载中…</div>
      ) : subs.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-ink-300 bg-white px-5 py-12 text-center text-sm text-ink-500">
          还没有开通任何服务。点击右上角「开通服务」从服务目录里挑一个开始。
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {subs.map((s) => (
            <ActivatedCard key={s.id} sub={s} />
          ))}
        </div>
      )}

      {pickerOpen ? (
        <CatalogPicker
          activatedSKUIds={new Set((subs ?? []).map((s) => s.sku_id))}
          onActivated={async () => {
            // 开通成功后立即刷新已开通列表 —— 卡片从模态消失（变"已开通"）
            // 同时主区多一张活跃订阅卡。
            try {
              const r = await api.get<{ data: Subscription[] }>('/api/user/subscriptions');
              setSubs(r.data ?? []);
            } catch (e) {
              setError((e as Error).message);
            }
          }}
          onClose={() => setPickerOpen(false)}
        />
      ) : null}
    </div>
  );
}

function ActivatedCard({ sub }: { sub: Subscription }) {
  const pct =
    sub.quota_total > 0 ? Math.min(100, (sub.quota_used / sub.quota_total) * 100) : 0;
  const tone = pct >= 90 ? 'bg-rose-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500';
  // 整张卡片导航到 /services/[sku_id]，在详情页里看 metadata + 在线测试。
  return (
    <Link
      href={`/services/${encodeURIComponent(sub.sku_id)}`}
      className="group block rounded-2xl border border-ink-200 bg-white p-5 transition hover:border-ink-300 hover:shadow-sm"
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold group-hover:text-brand-700">{sub.display_name ?? sub.sku_id}</div>
          <div className="mono text-[11px] text-ink-500">{sub.sku_id}</div>
        </div>
        <span className="inline-flex shrink-0 rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-medium text-emerald-700">
          已开通
        </span>
      </div>
      <div className="mt-3 text-xs text-ink-500">
        {sub.plan_name || sub.plan_kind} · {sub.capability ?? '—'} · 单位 {sub.billing_unit ?? '—'}
      </div>
      <div className="mt-4 space-y-1">
        <div className="flex items-baseline justify-between text-xs">
          <span className="text-ink-700">周期配额</span>
          <span className="mono text-ink-500">
            {sub.quota_used.toLocaleString()} / {sub.quota_total.toLocaleString()} {sub.billing_unit ?? ''}
          </span>
        </div>
        <div className="h-2 overflow-hidden rounded-full bg-ink-200/60">
          <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
        </div>
      </div>
      <div className="mt-4 flex items-center justify-between text-[11px] text-ink-500">
        <span>查看详情 / 在线测试</span>
        <span className="text-ink-400 transition group-hover:translate-x-0.5">→</span>
      </div>
    </Link>
  );
}

// CatalogPicker 是"开通服务"模态。左侧是平台大类树，右侧是该分支
// 下的可开通 SKU 卡片。当前只激活 "llm/chat" 分支；其它分类做灰态占位。
//
// activatedSKUIds 让卡片能区分"已开通"vs"未开通"，按钮形态切换。
// onActivated 在某条 SKU 成功开通后调用，由父组件刷新订阅列表。
function CatalogPicker({
  activatedSKUIds,
  onActivated,
  onClose,
}: {
  activatedSKUIds: Set<string>;
  onActivated: () => void | Promise<void>;
  onClose: () => void;
}) {
  const [cat, setCat] = useState<ServiceCatalog | null>(null);
  const [error, setError] = useState<string | null>(null);
  // 选中的 (categoryId, capabilityId)；默认聚焦在唯一开放的分支。
  const [sel, setSel] = useState<{ category: string; capability: string }>({
    category: 'llm',
    capability: 'chat',
  });

  useEffect(() => {
    api
      .get<ServiceCatalog>('/api/user/services/catalog')
      .then(setCat)
      .catch((e) => setError((e as Error).message));
  }, []);

  // 把后端的 services 按 (category_id, capability) 过滤一遍给右栏。
  const filtered = useMemo<CatalogService[]>(() => {
    if (!cat) return [];
    return cat.services.filter(
      (s) => s.category_id === sel.category && s.capability === sel.capability
    );
  }, [cat, sel]);

  // 把 capabilities 按 category_id 分组，避免在 JSX 里重复 filter。
  const subsByCategory = useMemo(() => {
    const m: Record<string, ServiceCatalog['capabilities']> = {};
    if (!cat) return m;
    for (const c of cat.capabilities) {
      (m[c.category_id] ??= []).push(c);
    }
    return m;
  }, [cat]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 px-4 py-8">
      <div className="flex h-full max-h-[640px] w-full max-w-5xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3">
          <div>
            <div className="text-base font-semibold">开通服务</div>
            <div className="text-[11px] text-ink-500">
              当前阶段：仅开放「大模型服务 → 文本大模型服务」板块。其余分类陆续开放。
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg border border-ink-300 px-3 py-1 text-xs hover:bg-ink-200/40"
          >
            关闭
          </button>
        </div>

        {error ? (
          <div className="border-b border-rose-200 bg-rose-50 px-5 py-2 text-xs text-rose-700">
            {error}
          </div>
        ) : null}

        <div className="grid min-h-0 flex-1 grid-cols-[220px_1fr]">
          {/* 左：分类树 */}
          <aside className="overflow-y-auto border-r border-ink-200 bg-ink-100/40 py-3">
            {cat === null ? (
              <div className="px-4 text-xs text-ink-500">加载中…</div>
            ) : (
              cat.categories.map((c) => {
                const subs = subsByCategory[c.id] ?? [];
                return (
                  <div key={c.id} className="px-2 pb-3">
                    <div className="px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-ink-500">
                      {c.name}
                    </div>
                    {subs.length === 0 ? (
                      <div className="px-3 py-1 text-[11px] text-ink-400">敬请期待</div>
                    ) : (
                      subs.map((s) => {
                        const active = sel.category === c.id && sel.capability === s.id;
                        return (
                          <button
                            key={s.id}
                            onClick={() => setSel({ category: c.id, capability: s.id })}
                            className={`block w-full rounded-lg px-3 py-1.5 text-left text-xs ${
                              active
                                ? 'bg-ink-900 text-white'
                                : 'text-ink-700 hover:bg-ink-200/60'
                            }`}
                          >
                            {labelForCapability(c.id, s.id, s.display_name)}
                          </button>
                        );
                      })
                    )}
                  </div>
                );
              })
            )}
            {/* 其它大类的占位 — 当前 catalog.Categories 已经只剩 llm，
                但显式列一行能让用户知道我们规划里还有别的。 */}
            <div className="px-2 pb-3">
              <div className="px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-ink-400">
                其它（敬请期待）
              </div>
              {['语音服务', '翻译服务', '视觉服务'].map((n) => (
                <div
                  key={n}
                  className="px-3 py-1 text-[11px] text-ink-400"
                >
                  {n}
                </div>
              ))}
            </div>
          </aside>

          {/* 右：SKU 卡片 */}
          <section className="overflow-y-auto px-5 py-4">
            {cat === null ? (
              <div className="text-sm text-ink-500">加载中…</div>
            ) : filtered.length === 0 ? (
              <div className="rounded-xl border border-dashed border-ink-300 bg-ink-100/40 px-4 py-12 text-center text-sm text-ink-500">
                当前分支没有可开通服务。
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                {filtered.map((s) => (
                  <SKUCard
                    key={s.id}
                    sku={s}
                    activated={activatedSKUIds.has(s.id)}
                    onActivated={onActivated}
                  />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}

function labelForCapability(categoryId: string, capId: string, fallback: string): string {
  // 给 chat 一个更贴近用户认知的子分类名："文本大模型服务"。
  if (categoryId === 'llm' && capId === 'chat') return '文本大模型服务';
  return fallback;
}

function SKUCard({
  sku,
  activated,
  onActivated,
}: {
  sku: CatalogService;
  activated: boolean;
  onActivated: () => void | Promise<void>;
}) {
  const hasPrice = sku.input_per_unit_cents !== undefined || sku.output_per_unit_cents !== undefined;
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function activate() {
    setBusy(true);
    setErr(null);
    try {
      await api.post('/api/user/subscriptions/activate', { sku_id: sku.id });
      await onActivated();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col rounded-xl border border-ink-200 bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">{sku.display_name}</div>
          <div className="mono text-[11px] text-ink-500">{sku.id}</div>
        </div>
        {sku.vendor_name ? (
          <span className="inline-flex shrink-0 rounded-full bg-brand-100 px-2 py-0.5 text-[10px] font-medium text-brand-700">
            {sku.vendor_name}
          </span>
        ) : null}
      </div>

      <div className="mt-2 text-[11px] text-ink-500">
        {sku.vendor_product_name ?? sku.vendor_product_id}
        {sku.upstream_model ? ` · ${sku.upstream_model}` : ''}
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] text-ink-500">
        <div>
          <div className="text-ink-400">计费单位</div>
          <div className="text-ink-700">{sku.billing_unit}</div>
        </div>
        {sku.context_window ? (
          <div>
            <div className="text-ink-400">上下文</div>
            <div className="text-ink-700">{sku.context_window.toLocaleString()}</div>
          </div>
        ) : null}
        {hasPrice ? (
          <>
            <div>
              <div className="text-ink-400">输入单价</div>
              <div className="text-ink-700">
                {sku.input_per_unit_cents !== undefined
                  ? `${sku.input_per_unit_cents.toFixed(2)} 分`
                  : '—'}
              </div>
            </div>
            <div>
              <div className="text-ink-400">输出单价</div>
              <div className="text-ink-700">
                {sku.output_per_unit_cents !== undefined
                  ? `${sku.output_per_unit_cents.toFixed(2)} 分`
                  : '—'}
              </div>
            </div>
          </>
        ) : null}
      </div>

      <div className="mt-4 flex items-center justify-between border-t border-ink-200 pt-3">
        <span className="text-[11px] text-ink-400">
          {err ? <span className="text-rose-600">{err}</span> : activated ? '已开通' : '点击立即开通'}
        </span>
        {activated ? (
          <span className="inline-flex rounded-lg bg-emerald-100 px-3 py-1 text-xs font-medium text-emerald-700">
            已开通
          </span>
        ) : (
          <button
            disabled={busy}
            onClick={activate}
            className="rounded-lg bg-ink-900 px-3 py-1 text-xs font-medium text-white hover:bg-ink-800 disabled:opacity-60"
          >
            {busy ? '开通中…' : '开通'}
          </button>
        )}
      </div>
    </div>
  );
}
