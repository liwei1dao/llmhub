'use client';

import { useEffect, useState } from 'react';
import { api, type Profile, type UsageBucket, type Wallet } from '@/lib/api';
import { fmtCents, fmtNumber } from '@/lib/format';

export default function DashboardPage() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [usage, setUsage] = useState<{ from: string; to: string; data: UsageBucket[] } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.get<Profile>('/api/user/profile'),
      api.get<Wallet>('/api/user/wallet'),
      api.get<{ from: string; to: string; data: UsageBucket[] }>('/api/user/usage/series?range=7d'),
    ])
      .then(([p, w, u]) => {
        setProfile(p);
        setWallet(w);
        setUsage(u);
      })
      .catch((err) => setError((err as Error).message));
  }, []);

  if (error) return <ErrorBanner message={error} />;
  if (!profile || !wallet || !usage) return <Loading />;

  const totals = usage.data.reduce(
    (acc, b) => ({
      calls: acc.calls + b.calls,
      cost: acc.cost + b.cost_retail_cents,
      tokens: acc.tokens + b.tokens_in + b.tokens_out,
    }),
    { calls: 0, cost: 0, tokens: 0 },
  );

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">总览</h1>
        <p className="mt-1 text-sm text-ink-500">
          {profile.email ?? profile.phone} · QPS 限额 {profile.qps_limit}
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
        <StatCard
          label="可用余额"
          value={fmtCents(wallet.balance_cents, wallet.currency)}
          hint={`冻结 ${fmtCents(wallet.frozen_cents, wallet.currency)}`}
        />
        <StatCard
          label="累计充值"
          value={fmtCents(wallet.total_recharged_cents, wallet.currency)}
        />
        <StatCard label="近 7 天调用" value={fmtNumber(totals.calls)} hint={`Token ${fmtNumber(totals.tokens)}`} />
        <StatCard label="近 7 天消费" value={fmtCents(totals.cost, wallet.currency)} />
      </div>

      <UsageTable data={usage.data} />
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5 shadow-sm">
      <div className="text-sm text-ink-500">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight">{value}</div>
      {hint ? <div className="mt-1 text-xs text-ink-500">{hint}</div> : null}
    </div>
  );
}

function UsageTable({ data }: { data: UsageBucket[] }) {
  if (data.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-ink-200 bg-white p-12 text-center text-sm text-ink-500">
        近 7 天还没有调用 — 去 <a className="text-brand-600 underline" href="/api-keys">创建 API Key</a> 开始第一次调用。
      </div>
    );
  }
  return (
    <div className="overflow-hidden rounded-2xl border border-ink-200 bg-white">
      <div className="border-b border-ink-200 px-5 py-3 text-sm font-medium">近 7 天用量</div>
      <table className="w-full text-sm">
        <thead className="bg-ink-200/30 text-xs uppercase tracking-wider text-ink-500">
          <tr>
            <th className="px-5 py-2.5 text-left">日期</th>
            <th className="px-5 py-2.5 text-left">能力</th>
            <th className="px-5 py-2.5 text-right">调用</th>
            <th className="px-5 py-2.5 text-right">成功</th>
            <th className="px-5 py-2.5 text-right">Token</th>
            <th className="px-5 py-2.5 text-right">消费</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-ink-200">
          {data.map((b) => (
            <tr key={`${b.day}-${b.capability_id}`}>
              <td className="px-5 py-2.5 mono text-ink-700">{b.day}</td>
              <td className="px-5 py-2.5">{b.capability_id}</td>
              <td className="px-5 py-2.5 text-right">{fmtNumber(b.calls)}</td>
              <td className="px-5 py-2.5 text-right">{fmtNumber(b.success_calls)}</td>
              <td className="px-5 py-2.5 text-right">{fmtNumber(b.tokens_in + b.tokens_out)}</td>
              <td className="px-5 py-2.5 text-right">{fmtCents(b.cost_retail_cents)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Loading() {
  return <div className="text-sm text-ink-500">加载中…</div>;
}
function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
      请求失败：{message}
      <span className="ml-2 text-rose-500">（账号未登录会跳转 /login，可能是后端未启动）</span>
    </div>
  );
}
