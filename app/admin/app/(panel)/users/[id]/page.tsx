'use client';

import { useEffect, useMemo, useState } from 'react';
import { useParams, useRouter, useSearchParams } from 'next/navigation';
import {
  api,
  type AdminSubscription,
  type AdminUserDetail,
  type AdminUserUsage,
  type AdminUserWallet,
  type AdminUsageBySKU,
  type AdminUsageDailyBucket,
  type AdminUsageRecentCall,
  type AdminUsageStatusBucket,
  type GrantSubscriptionReq,
  type PlatformService,
} from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../../_components/page-header';

// 用户详情：用 tab 分流
//   - overview      ：用户信息 / 钱包 / 订阅一览
//   - subscriptions ：订阅明细 + 开订阅表单 + 配额 / 暂停 / 取消操作
//   - usage         ：使用统计（KPI / 按 SKU 拆 / 状态拆 / 日趋势 / 近期调用）
//
// 之前所有信息堆一页，运营要看用量得自己脑补 SQL；拆 tab 后默认页只
// 展示「这个用户是谁 / 余额多少 / 订阅了哪些 SKU」三句话能讲完的内容，
// 用量 / 订阅明细按需点进去。

type Tab = 'overview' | 'subscriptions' | 'usage';

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const search = useSearchParams();
  const tab = (search.get('tab') ?? 'overview') as Tab;

  const [user, setUser] = useState<AdminUserDetail | null>(null);
  const [wallet, setWallet] = useState<AdminUserWallet | null>(null);
  const [subs, setSubs] = useState<AdminSubscription[]>([]);
  const [skus, setSkus] = useState<PlatformService[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const [u, w, s, k] = await Promise.all([
        api.get<AdminUserDetail>(`/api/admin/users/${id}`),
        api.get<AdminUserWallet>(`/api/admin/users/${id}/wallet`),
        api.get<{ data: AdminSubscription[] }>(`/api/admin/users/${id}/subscriptions`),
        api.get<{ data: PlatformService[] }>(`/api/admin/platform-services?limit=500`),
      ]);
      setUser(u);
      setWallet(w);
      setSubs(s.data ?? []);
      setSkus(k.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  function switchTab(next: Tab) {
    const qs = next === 'overview' ? '' : `?tab=${next}`;
    router.push(`/users/${id}${qs}`, { scroll: false });
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={user ? `用户 #${user.id}` : '加载中…'}
        subtitle={user?.email ?? user?.phone ?? ''}
      />

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      {user ? <Tabs current={tab} onChange={switchTab} /> : null}

      {tab === 'overview' && user ? (
        <OverviewTab user={user} wallet={wallet} subs={subs} onJumpUsage={() => switchTab('usage')} onJumpSubs={() => switchTab('subscriptions')} />
      ) : null}

      {tab === 'subscriptions' && user ? (
        <SubscriptionsTab userId={user.id} subs={subs} skus={skus} onReload={load} />
      ) : null}

      {tab === 'usage' && user ? <UsageTab userId={user.id} /> : null}
    </div>
  );
}

function Tabs({ current, onChange }: { current: Tab; onChange: (t: Tab) => void }) {
  const items: { id: Tab; label: string }[] = [
    { id: 'overview', label: '概览' },
    { id: 'subscriptions', label: '订阅' },
    { id: 'usage', label: '使用统计' },
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

function OverviewTab({
  user,
  wallet,
  subs,
  onJumpUsage,
  onJumpSubs,
}: {
  user: AdminUserDetail;
  wallet: AdminUserWallet | null;
  subs: AdminSubscription[];
  onJumpUsage: () => void;
  onJumpSubs: () => void;
}) {
  return (
    <div className="space-y-6">
      <UserCard u={user} />
      {wallet ? <WalletCard w={wallet} /> : null}

      <div className="rounded-2xl border border-ink-200 bg-white">
        <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3">
          <div>
            <div className="text-sm font-semibold text-ink-900">订阅速览</div>
            <div className="text-[11px] text-ink-500">仅展示最多 5 条 active 订阅，全部请去「订阅」tab。</div>
          </div>
          <div className="flex items-center gap-3 text-xs">
            <button onClick={onJumpSubs} className="text-brand-700 hover:underline">
              管理订阅 →
            </button>
            <button onClick={onJumpUsage} className="text-brand-700 hover:underline">
              查看用量 →
            </button>
          </div>
        </div>
        {subs.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-ink-500">尚无订阅</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
              <tr>
                <th className="px-4 py-2 text-left">SKU</th>
                <th className="px-4 py-2 text-right">配额</th>
                <th className="px-4 py-2 text-left">周期截止</th>
                <th className="px-4 py-2 text-left">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-200">
              {subs
                .filter((s) => s.status === 'active')
                .slice(0, 5)
                .map((s) => (
                  <tr key={s.id}>
                    <td className="px-4 py-2.5">
                      <div className="text-ink-800">{s.sku_id}</div>
                      {s.plan_name ? <div className="text-[11px] text-ink-500">{s.plan_name}</div> : null}
                    </td>
                    <td className="px-4 py-2.5 mono text-right text-xs">
                      {s.quota_used.toLocaleString()} / {s.quota_total.toLocaleString()}
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(s.period_end)}</td>
                    <td className="px-4 py-2.5">
                      <SubStatusPill status={s.status} />
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function UserCard({ u }: { u: AdminUserDetail }) {
  return (
    <div className="grid grid-cols-2 gap-4 rounded-2xl border border-ink-200 bg-white p-5 md:grid-cols-4">
      <Field label="状态" value={u.status} />
      <Field label="风险分" value={String(u.risk_score)} />
      <Field label="QPS 上限" value={String(u.qps_limit)} />
      <Field label="日消费上限" value={fmtCents(u.daily_spend_limit_cents)} />
      <Field label="注册时间" value={fmtDateTime(u.created_at)} />
      <Field label="最近登录" value={fmtDateTime(u.last_login_at)} />
      <Field label="实名等级" value={String(u.realname_level ?? 0)} />
      <Field label="昵称" value={u.display_name ?? '-'} />
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11px] text-ink-500">{label}</div>
      <div className="text-sm text-ink-800">{value}</div>
    </div>
  );
}

function WalletCard({ w }: { w: AdminUserWallet }) {
  if (!w.account_exists) {
    return (
      <div className="rounded-2xl border border-ink-200 bg-white p-5">
        <div className="mb-2 text-sm font-semibold text-ink-900">💰 钱包</div>
        <div className="text-xs text-ink-500">用户尚未发起过任何充值，钱包账户未创建。</div>
      </div>
    );
  }
  const currency = w.currency || 'CNY';
  return (
    <div className="rounded-2xl border border-ink-200 bg-white">
      <div className="border-b border-ink-200 px-5 py-3 text-sm font-semibold text-ink-900">
        💰 钱包 · 消费（最近 30 日）
      </div>
      <div className="grid grid-cols-2 gap-4 px-5 py-4 md:grid-cols-5">
        <Stat label="可用余额" value={fmtCents(w.balance_cents ?? 0, currency)} tone="emerald" />
        <Stat label="冻结" value={fmtCents(w.frozen_cents ?? 0, currency)} />
        <Stat label="累计充值" value={fmtCents(w.total_recharged_cents ?? 0, currency)} />
        <Stat label="累计消费" value={fmtCents(w.total_spent_cents ?? 0, currency)} />
        <Stat
          label="近 30 日消费"
          value={fmtCents(w.spent_30d_cents ?? 0, currency)}
          sub={`${(w.calls_30d ?? 0).toLocaleString()} 次调用`}
        />
      </div>

      <div className="border-t border-ink-200">
        <div className="px-5 py-2.5 text-xs uppercase tracking-wider text-ink-500">
          最近充值（{w.recharges.length}）
        </div>
        {w.recharges.length === 0 ? (
          <div className="px-5 pb-4 pt-1 text-sm text-ink-500">—</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
              <tr>
                <th className="px-4 py-2 text-left">订单号</th>
                <th className="px-4 py-2 text-left">渠道</th>
                <th className="px-4 py-2 text-right">金额</th>
                <th className="px-4 py-2 text-left">状态</th>
                <th className="px-4 py-2 text-left">支付时间</th>
                <th className="px-4 py-2 text-left">创建</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-200">
              {w.recharges.map((r) => (
                <tr key={r.order_no}>
                  <td className="px-4 py-2 mono text-xs text-ink-500">{r.order_no}</td>
                  <td className="px-4 py-2 text-xs">{r.channel}</td>
                  <td className="px-4 py-2 text-right tabular-nums">{fmtCents(r.amount_cents, currency)}</td>
                  <td className="px-4 py-2">
                    <RechargeStatusPill status={r.status} />
                  </td>
                  <td className="px-4 py-2 text-xs text-ink-500">{r.paid_at ? fmtDateTime(r.paid_at) : '-'}</td>
                  <td className="px-4 py-2 text-xs text-ink-500">{fmtDateTime(r.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function Stat({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone?: 'emerald' }) {
  const valueCls = tone === 'emerald' ? 'text-emerald-700' : 'text-ink-900';
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wider text-ink-500">{label}</div>
      <div className={`mt-0.5 text-base font-medium tabular-nums ${valueCls}`}>{value}</div>
      {sub ? <div className="text-[11px] text-ink-500">{sub}</div> : null}
    </div>
  );
}

function RechargeStatusPill({ status }: { status: string }) {
  const tone =
    status === 'paid'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'pending'
        ? 'bg-amber-100 text-amber-700'
        : status === 'failed' || status === 'cancelled' || status === 'refunded'
          ? 'bg-rose-100 text-rose-700'
          : 'bg-ink-100 text-ink-500';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] ${tone}`}>{status}</span>;
}

function SubStatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'suspended'
        ? 'bg-amber-100 text-amber-700'
        : status === 'cancelled'
          ? 'bg-rose-100 text-rose-700'
          : 'bg-ink-100 text-ink-800';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}

// ─────────────────── Subscriptions tab ───────────────────

function SubscriptionsTab({
  userId,
  subs,
  skus,
  onReload,
}: {
  userId: number;
  subs: AdminSubscription[];
  skus: PlatformService[];
  onReload: () => void | Promise<void>;
}) {
  const [showGrant, setShowGrant] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onCancel(s: AdminSubscription) {
    if (!confirm(`取消订阅 ${s.sku_id}? 该用户的 SDK 立即拿不到新 lease。`)) return;
    try {
      await api.delete(`/api/admin/subscriptions/${s.id}`);
      await onReload();
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  async function onAddQuota(s: AdminSubscription) {
    const raw = prompt(`为 ${s.sku_id} 新增多少配额？（当前 quota_total=${s.quota_total}）`);
    if (!raw) return;
    const add = Number(raw);
    if (!Number.isFinite(add) || add <= 0) {
      alert('请输入一个正数');
      return;
    }
    try {
      await api.patch(`/api/admin/subscriptions/${s.id}`, { quota_total: s.quota_total + add });
      await onReload();
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  async function onToggleStatus(s: AdminSubscription) {
    const target = s.status === 'active' ? 'suspended' : 'active';
    try {
      await api.patch(`/api/admin/subscriptions/${s.id}`, { status: target });
      await onReload();
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  return (
    <div className="space-y-4">
      {err ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{err}</div>
      ) : null}

      <div className="rounded-2xl border border-ink-200 bg-white">
        <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3">
          <div className="text-sm font-semibold text-ink-900">订阅（{subs.length}）</div>
          <button
            onClick={() => setShowGrant((v) => !v)}
            className="rounded-lg border border-ink-200 bg-white px-4 py-1.5 text-xs font-medium text-ink-900 hover:bg-ink-50"
          >
            {showGrant ? '收起' : '+ 开订阅'}
          </button>
        </div>

        {showGrant ? (
          <GrantForm
            userId={userId}
            skus={skus}
            onClose={() => setShowGrant(false)}
            onCreated={() => {
              setShowGrant(false);
              onReload();
            }}
          />
        ) : null}

        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">SKU</th>
              <th className="px-4 py-2.5 text-left">套餐</th>
              <th className="px-4 py-2.5 text-right">配额（已用 / 总）</th>
              <th className="px-4 py-2.5 text-right">QPS</th>
              <th className="px-4 py-2.5 text-left">周期截止</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {subs.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-10 text-center text-ink-500">
                  尚无订阅 — 用上方 + 开订阅 按钮创建一条
                </td>
              </tr>
            ) : (
              subs.map((s) => (
                <tr key={s.id}>
                  <td className="px-4 py-2.5">
                    <div className="text-ink-800">{s.sku_id}</div>
                    {s.plan_name ? <div className="text-[11px] text-ink-500">{s.plan_name}</div> : null}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{s.plan_kind}</td>
                  <td className="px-4 py-2.5 mono text-right text-xs">
                    {s.quota_used.toLocaleString()} / {s.quota_total.toLocaleString()}
                  </td>
                  <td className="px-4 py-2.5 mono text-right text-xs">{s.qps_limit}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(s.period_end)}</td>
                  <td className="px-4 py-2.5">
                    <SubStatusPill status={s.status} />
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <div className="inline-flex flex-wrap justify-end gap-1.5">
                      <button
                        onClick={() => onAddQuota(s)}
                        disabled={s.status !== 'active'}
                        className="rounded-md border border-ink-300 px-2 py-0.5 text-[11px] text-ink-800 hover:bg-ink-100 disabled:opacity-40"
                      >
                        加配额
                      </button>
                      <button
                        onClick={() => onToggleStatus(s)}
                        className="rounded-md border border-ink-300 px-2 py-0.5 text-[11px] text-ink-800 hover:bg-ink-100"
                      >
                        {s.status === 'active' ? '暂停' : '恢复'}
                      </button>
                      <button
                        onClick={() => onCancel(s)}
                        disabled={s.status === 'cancelled'}
                        className="rounded-md border border-rose-300 px-2 py-0.5 text-[11px] text-rose-700 hover:bg-rose-50 disabled:opacity-40"
                      >
                        取消
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─────────────────── Usage tab ───────────────────

function UsageTab({ userId }: { userId: number }) {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<AdminUserUsage | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setErr(null);
    try {
      const r = await api.get<AdminUserUsage>(
        `/api/admin/users/${userId}/usage?days=${days}&limit_recent=50`,
      );
      setData(r);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [userId, days]);

  const totals = data?.totals;
  const successRate = useMemo(() => {
    if (!totals || totals.calls === 0) return null;
    return (totals.success_calls / totals.calls) * 100;
  }, [totals]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div className="flex gap-2 rounded-lg border border-ink-200 bg-white p-0.5 text-xs">
          {[1, 7, 30, 90].map((n) => (
            <button
              key={n}
              onClick={() => setDays(n)}
              className={`rounded-md px-3 py-1 transition ${
                days === n ? 'bg-ink-900 text-white' : 'text-ink-600 hover:bg-ink-100'
              }`}
            >
              {n === 1 ? '今日' : `${n} 天`}
            </button>
          ))}
        </div>
        <div className="text-[11px] text-ink-500">
          {data ? `${data.from} → ${data.to}` : null}
          {loading ? '（加载中…）' : null}
        </div>
      </div>

      {err ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{err}</div>
      ) : null}

      {/* ── KPI 行 ── */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-6">
        <KPI label="调用总数" value={totals ? totals.calls.toLocaleString() : '—'} />
        <KPI
          label="成功率"
          value={successRate != null ? `${successRate.toFixed(1)}%` : '—'}
          tone={successRate != null && successRate < 80 ? 'rose' : successRate != null && successRate < 95 ? 'amber' : 'emerald'}
        />
        <KPI
          label="平均时延"
          value={totals ? `${Math.round(totals.avg_latency_ms)}ms` : '—'}
          sub={totals ? `TTFB ${Math.round(totals.avg_ttfb_ms)}ms` : undefined}
        />
        <KPI label="使用 SKU 数" value={totals ? totals.unique_skus.toLocaleString() : '—'} />
        <KPI
          label="累计 tokens"
          value={totals ? (totals.tokens_in + totals.tokens_out).toLocaleString() : '—'}
          sub={totals ? `in ${totals.tokens_in.toLocaleString()} · out ${totals.tokens_out.toLocaleString()}` : undefined}
        />
        <KPI
          label="期间消费"
          value={totals ? fmtCents(totals.cost_retail_cents) : '—'}
        />
      </div>

      {/* ── 双列：左每日趋势 + 状态拆分；右 SKU 拆分 ── */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1fr_420px]">
        <DailyChart daily={data?.daily ?? []} from={data?.from} to={data?.to} />
        <StatusBreakdown buckets={data?.by_status ?? []} totalCalls={totals?.calls ?? 0} />
      </div>

      <SKUBreakdown rows={data?.by_sku ?? []} />

      <RecentCalls rows={data?.recent ?? []} />
    </div>
  );
}

function KPI({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: 'emerald' | 'amber' | 'rose';
}) {
  const cls =
    tone === 'rose'
      ? 'text-rose-700'
      : tone === 'amber'
        ? 'text-amber-700'
        : tone === 'emerald'
          ? 'text-emerald-700'
          : 'text-ink-900';
  return (
    <div className="rounded-xl border border-ink-200 bg-white px-3 py-2.5">
      <div className="text-[11px] uppercase tracking-wider text-ink-500">{label}</div>
      <div className={`mono mt-1 text-base font-semibold tabular-nums ${cls}`}>{value}</div>
      {sub ? <div className="text-[11px] text-ink-500">{sub}</div> : null}
    </div>
  );
}

// DailyChart 用纯 div 做的柱状图。x 轴 = 天（按 [from, to) 补零），
// y 轴 = 当日 calls。带 hover tooltip。引入 chart 库代价太高，30 根
// 柱子的简单图自己画足够。
function DailyChart({
  daily,
  from,
  to,
}: {
  daily: AdminUsageDailyBucket[];
  from?: string;
  to?: string;
}) {
  // 把 daily 索引化，按天补零。
  const map = new Map(daily.map((d) => [d.day, d]));
  const days: AdminUsageDailyBucket[] = [];
  if (from && to) {
    const start = new Date(from + 'T00:00:00Z');
    const end = new Date(to + 'T00:00:00Z'); // exclusive
    for (let d = new Date(start); d < end; d.setUTCDate(d.getUTCDate() + 1)) {
      const key = d.toISOString().slice(0, 10);
      days.push(
        map.get(key) ?? {
          day: key,
          calls: 0,
          success_calls: 0,
          tokens_in: 0,
          tokens_out: 0,
          cost_retail_cents: 0,
        },
      );
    }
  } else {
    days.push(...daily);
  }
  const max = Math.max(1, ...days.map((d) => d.calls));

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-semibold text-ink-900">每日调用走势</div>
        <div className="text-[11px] text-ink-500">{days.length} 天 · 峰值 {max.toLocaleString()}</div>
      </div>
      {days.length === 0 ? (
        <div className="rounded-xl border border-dashed border-ink-300 bg-ink-100/30 px-4 py-10 text-center text-xs text-ink-500">
          区间内没有任何调用。
        </div>
      ) : (
        <div className="flex h-40 items-end gap-[2px]">
          {days.map((d, i) => {
            const h = max > 0 ? (d.calls / max) * 100 : 0;
            const failed = d.calls - d.success_calls;
            const successH = d.calls > 0 ? (d.success_calls / d.calls) * h : 0;
            const failedH = h - successH;
            return (
              <div
                key={i}
                title={`${d.day}\n调用 ${d.calls}（成功 ${d.success_calls} / 失败 ${failed}）\n消费 ${fmtCents(d.cost_retail_cents)}`}
                className="group relative flex flex-1 flex-col-reverse"
                style={{ minWidth: 4 }}
              >
                {/* success 部分（下层）+ failed 部分（上层，rose 色） */}
                <div className="w-full bg-emerald-500/80 transition group-hover:bg-emerald-600" style={{ height: `${successH}%` }} />
                {failedH > 0 ? (
                  <div className="w-full bg-rose-500/80" style={{ height: `${failedH}%` }} />
                ) : null}
              </div>
            );
          })}
        </div>
      )}
      <div className="mt-2 flex items-center gap-3 text-[11px] text-ink-500">
        <Legend color="bg-emerald-500/80" label="成功" />
        <Legend color="bg-rose-500/80" label="失败" />
      </div>
    </div>
  );
}

function Legend({ color, label }: { color: string; label: string }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className={`inline-block h-2 w-2 rounded-sm ${color}`} />
      <span>{label}</span>
    </div>
  );
}

function StatusBreakdown({ buckets, totalCalls }: { buckets: AdminUsageStatusBucket[]; totalCalls: number }) {
  if (totalCalls === 0) {
    return (
      <div className="rounded-2xl border border-ink-200 bg-white p-5">
        <div className="mb-3 text-sm font-semibold text-ink-900">状态分布</div>
        <div className="rounded-xl border border-dashed border-ink-300 bg-ink-100/30 px-4 py-10 text-center text-xs text-ink-500">
          没有数据。
        </div>
      </div>
    );
  }
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="mb-3 text-sm font-semibold text-ink-900">状态分布</div>
      <div className="space-y-2">
        {buckets.map((b) => {
          const pct = (b.count / totalCalls) * 100;
          const tone =
            b.status === 'success'
              ? 'bg-emerald-500'
              : b.status === 'rate_limited'
                ? 'bg-amber-500'
                : b.status === 'auth_failed'
                  ? 'bg-fuchsia-500'
                  : b.status === 'timeout'
                    ? 'bg-orange-500'
                    : 'bg-rose-500';
          return (
            <div key={b.status}>
              <div className="flex items-baseline justify-between text-xs">
                <span className="text-ink-700">{b.status}</span>
                <span className="mono text-ink-500">
                  {b.count.toLocaleString()} ({pct.toFixed(1)}%)
                </span>
              </div>
              <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-ink-100">
                <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function SKUBreakdown({ rows }: { rows: AdminUsageBySKU[] }) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white">
      <div className="border-b border-ink-200 px-5 py-3 text-sm font-semibold text-ink-900">
        按 SKU 拆分（Top {rows.length}）
      </div>
      {rows.length === 0 ? (
        <div className="px-5 py-10 text-center text-sm text-ink-500">区间内没有调用记录。</div>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2 text-left">SKU</th>
              <th className="px-4 py-2 text-left">vendor / product</th>
              <th className="px-4 py-2 text-right">调用</th>
              <th className="px-4 py-2 text-right">成功率</th>
              <th className="px-4 py-2 text-right">tokens (in / out)</th>
              <th className="px-4 py-2 text-right">消费</th>
              <th className="px-4 py-2 text-left">最后调用</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {rows.map((r, i) => {
              const succ = r.calls > 0 ? (r.success_calls / r.calls) * 100 : 0;
              return (
                <tr key={`${r.sku_id}-${i}`}>
                  <td className="px-4 py-2.5 mono text-xs text-ink-800">{r.sku_id || '—'}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">
                    {r.vendor_id} / {r.product_id}
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums">{r.calls.toLocaleString()}</td>
                  <td
                    className={`px-4 py-2.5 text-right text-xs tabular-nums ${
                      succ < 80 ? 'text-rose-700' : succ < 95 ? 'text-amber-700' : 'text-emerald-700'
                    }`}
                  >
                    {succ.toFixed(1)}%
                  </td>
                  <td className="px-4 py-2.5 text-right text-xs tabular-nums text-ink-700">
                    {r.tokens_in.toLocaleString()} / {r.tokens_out.toLocaleString()}
                  </td>
                  <td className="px-4 py-2.5 text-right tabular-nums">{fmtCents(r.cost_retail_cents)}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(r.last_used_at)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
}

function RecentCalls({ rows }: { rows: AdminUsageRecentCall[] }) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white">
      <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3">
        <div className="text-sm font-semibold text-ink-900">近期调用（最多 50 条）</div>
        <div className="text-[11px] text-ink-500">来自 metering.call_logs；不展示 prompt / 响应内容（平台无法获知）</div>
      </div>
      {rows.length === 0 ? (
        <div className="px-5 py-10 text-center text-sm text-ink-500">还没有任何调用记录。</div>
      ) : (
        <div className="max-h-[480px] overflow-auto">
          <table className="w-full text-sm">
            <thead className="sticky top-0 bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
              <tr>
                <th className="px-4 py-2 text-left">时间</th>
                <th className="px-4 py-2 text-left">SKU</th>
                <th className="px-4 py-2 text-left">vendor</th>
                <th className="px-4 py-2 text-left">状态</th>
                <th className="px-4 py-2 text-right">耗时</th>
                <th className="px-4 py-2 text-right">TTFB</th>
                <th className="px-4 py-2 text-right">tokens</th>
                <th className="px-4 py-2 text-left">错误</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-200">
              {rows.map((r) => (
                <tr key={r.request_id || r.ts}>
                  <td className="px-4 py-2 text-xs text-ink-700">{fmtDateTime(r.ts)}</td>
                  <td className="px-4 py-2 mono text-xs text-ink-800">{r.sku_id || '—'}</td>
                  <td className="px-4 py-2 text-xs text-ink-500">{r.vendor_id || '—'}</td>
                  <td className="px-4 py-2">
                    <CallStatusPill status={r.status} />
                  </td>
                  <td className="px-4 py-2 text-right text-xs tabular-nums text-ink-700">
                    {r.duration_ms ? `${r.duration_ms}ms` : '—'}
                  </td>
                  <td className="px-4 py-2 text-right text-xs tabular-nums text-ink-500">
                    {r.ttfb_ms ? `${r.ttfb_ms}ms` : '—'}
                  </td>
                  <td className="px-4 py-2 text-right text-xs tabular-nums text-ink-700">
                    {(r.tokens_in + r.tokens_out).toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-xs text-rose-700">{r.error_code || ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function CallStatusPill({ status }: { status: string }) {
  const tone =
    status === 'success'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'rate_limited'
        ? 'bg-amber-100 text-amber-700'
        : status === 'auth_failed'
          ? 'bg-fuchsia-100 text-fuchsia-700'
          : status === 'timeout'
            ? 'bg-orange-100 text-orange-700'
            : 'bg-rose-100 text-rose-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] ${tone}`}>{status}</span>;
}

// ─────────────────── Grant form (shared by Subscriptions tab) ───────────────────

function GrantForm({
  userId,
  skus,
  onClose,
  onCreated,
}: {
  userId: number;
  skus: PlatformService[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState<Partial<GrantSubscriptionReq>>({
    plan_kind: 'monthly',
    quota_total: 100000,
    qps_limit: 10,
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const skuOptions = skus.filter((s) => s.status === 'active' && s.is_public);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.sku_id) {
      setError('请选择 SKU');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await api.post(`/api/admin/users/${userId}/subscriptions`, {
        sku_id: form.sku_id,
        plan_kind: form.plan_kind ?? 'monthly',
        plan_name: form.plan_name,
        quota_total: form.quota_total ?? 100000,
        period_end: form.period_end,
        auto_renew: form.auto_renew ?? false,
        qps_limit: form.qps_limit ?? 10,
        daily_quota_limit: form.daily_quota_limit,
      } as GrantSubscriptionReq);
      onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="border-b border-ink-200 bg-ink-50 p-5">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <Select
          label="SKU *"
          value={form.sku_id ?? ''}
          onChange={(v) => setForm({ ...form, sku_id: v })}
          options={[
            { value: '', label: '— 请选择 —' },
            ...skuOptions.map((s) => ({ value: s.id, label: `${s.id} · ${s.display_name}` })),
          ]}
        />
        <Select
          label="套餐类型 *"
          value={form.plan_kind ?? 'monthly'}
          onChange={(v) => setForm({ ...form, plan_kind: v as GrantSubscriptionReq['plan_kind'] })}
          options={[
            { value: 'monthly', label: 'monthly · 包月（默认 30d）' },
            { value: 'prepaid', label: 'prepaid · 预付费（默认 90d）' },
            { value: 'trial', label: 'trial · 试用（默认 7d）' },
          ]}
        />
        <Field2
          label="套餐名（展示用）"
          value={form.plan_name ?? ''}
          onChange={(v) => setForm({ ...form, plan_name: v })}
          placeholder="DeepSeek Chat 标准版"
        />
        <NumberField2
          label="quota_total（按 SKU 计费单位）"
          value={form.quota_total}
          onChange={(v) => setForm({ ...form, quota_total: v })}
        />
        <NumberField2
          label="QPS 上限"
          value={form.qps_limit}
          onChange={(v) => setForm({ ...form, qps_limit: v })}
        />
        <NumberField2
          label="每日配额上限（可空）"
          value={form.daily_quota_limit}
          onChange={(v) => setForm({ ...form, daily_quota_limit: v })}
        />
        <Field2
          label="周期截止 (RFC3339, 可空)"
          value={form.period_end ?? ''}
          onChange={(v) => setForm({ ...form, period_end: v })}
          placeholder="2026-12-01T00:00:00Z"
        />
        <label className="flex items-center gap-2 text-sm text-ink-800">
          <input
            type="checkbox"
            checked={form.auto_renew ?? false}
            onChange={(e) => setForm({ ...form, auto_renew: e.target.checked })}
          />
          自动续订
        </label>
      </div>

      {error ? <div className="mt-3 text-sm text-rose-700">{error}</div> : null}

      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-lg border border-ink-200 px-4 py-2 text-sm text-ink-800 hover:bg-ink-100"
        >
          取消
        </button>
        <button
          type="submit"
          disabled={submitting || !form.sku_id}
          className="rounded-lg border border-ink-200 bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-50 disabled:opacity-60"
        >
          {submitting ? '创建中…' : '开订阅'}
        </button>
      </div>
    </form>
  );
}

function Field2({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500"
      />
    </label>
  );
}

function NumberField2({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number | undefined;
  onChange: (v: number | undefined) => void;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        type="number"
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value ? Number(e.target.value) : undefined)}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500"
      />
    </label>
  );
}

function Select({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}
