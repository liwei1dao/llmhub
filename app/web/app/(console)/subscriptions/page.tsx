'use client';

import { useEffect, useState } from 'react';
import { api, type Subscription } from '@/lib/api';
import { fmtDateTime } from '@/lib/format';

// 服务订阅 + 配额观察 — 一个页面把两件事合到一起，因为它们是一个数据源
// （iam.subscriptions），分两页只是给用户多点一次。
export default function SubscriptionsPage() {
  const [rows, setRows] = useState<Subscription[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: Subscription[] }>('/api/user/subscriptions')
      .then((r) => setRows(r.data ?? []))
      .catch((e) => setError((e as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">服务订阅</h1>
        <p className="mt-1 text-sm text-ink-500">
          已订阅的服务（SKU）+ 当前周期内的配额使用情况。SDK 用 API
          Key + sku_id 调 <code className="mono rounded bg-ink-200/50 px-1">/sdk/credentials/issue</code>
          时，会按这里的配额做准入校验。
        </p>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800">
          {error}
        </div>
      ) : null}

      {rows === null ? (
        <div className="text-sm text-ink-500">加载中…</div>
      ) : rows.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-ink-300 bg-white px-5 py-12 text-center text-sm text-ink-500">
          还没有任何订阅。请到「服务列表」页查看可开通服务。
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {rows.map((s) => (
            <SubscriptionCard key={s.id} sub={s} />
          ))}
        </div>
      )}
    </div>
  );
}

function SubscriptionCard({ sub }: { sub: Subscription }) {
  const totalPct = sub.quota_total > 0 ? Math.min(100, (sub.quota_used / sub.quota_total) * 100) : 0;
  const dailyPct =
    sub.daily_quota_limit && sub.daily_quota_limit > 0
      ? Math.min(100, (sub.daily_used / sub.daily_quota_limit) * 100)
      : null;

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-sm font-semibold">{sub.display_name ?? sub.sku_id}</div>
          <div className="mono text-[11px] text-ink-500">{sub.sku_id}</div>
        </div>
        <PlanBadge kind={sub.plan_kind} />
      </div>

      <div className="mt-3 text-xs text-ink-500">
        {sub.plan_name || sub.plan_kind} · {sub.capability ?? '—'} · 单位 {sub.billing_unit ?? '—'}
      </div>

      <div className="mt-4 space-y-3">
        <Meter
          label="周期内配额"
          used={sub.quota_used}
          total={sub.quota_total}
          pct={totalPct}
          unit={sub.billing_unit}
        />
        {dailyPct !== null ? (
          <Meter
            label="今日配额"
            used={sub.daily_used}
            total={sub.daily_quota_limit ?? 0}
            pct={dailyPct}
            unit={sub.billing_unit}
          />
        ) : (
          <div className="text-[11px] text-ink-500">每日配额：不限</div>
        )}
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3 border-t border-ink-200 pt-3 text-[11px] text-ink-500">
        <div>
          <div className="text-ink-400">周期</div>
          <div className="text-ink-700">
            {fmtDateTime(sub.period_start)} → {fmtDateTime(sub.period_end)}
          </div>
        </div>
        <div>
          <div className="text-ink-400">QPS 上限</div>
          <div className="text-ink-700">{sub.qps_limit}</div>
        </div>
        {sub.input_per_unit_cents !== undefined ? (
          <div>
            <div className="text-ink-400">单价（输入）</div>
            <div className="text-ink-700">{sub.input_per_unit_cents.toFixed(2)} 分</div>
          </div>
        ) : null}
        {sub.output_per_unit_cents !== undefined ? (
          <div>
            <div className="text-ink-400">单价（输出）</div>
            <div className="text-ink-700">{sub.output_per_unit_cents.toFixed(2)} 分</div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function Meter({
  label,
  used,
  total,
  pct,
  unit,
}: {
  label: string;
  used: number;
  total: number;
  pct: number;
  unit?: string;
}) {
  const tone = pct >= 90 ? 'bg-rose-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500';
  return (
    <div>
      <div className="flex items-baseline justify-between text-xs">
        <span className="text-ink-700">{label}</span>
        <span className="mono text-ink-500">
          {used.toLocaleString()} / {total.toLocaleString()} {unit ?? ''}
        </span>
      </div>
      <div className="mt-1 h-2 overflow-hidden rounded-full bg-ink-200/60">
        <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function PlanBadge({ kind }: { kind: string }) {
  const tone =
    kind === 'monthly'
      ? 'bg-brand-100 text-brand-700'
      : kind === 'prepaid'
        ? 'bg-fuchsia-100 text-fuchsia-700'
        : kind === 'trial'
          ? 'bg-amber-100 text-amber-700'
          : 'bg-ink-200 text-ink-700';
  const label = kind === 'monthly' ? '包月' : kind === 'prepaid' ? '预付费' : kind === 'trial' ? '试用' : kind;
  return (
    <span className={`inline-flex shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium ${tone}`}>
      {label}
    </span>
  );
}
