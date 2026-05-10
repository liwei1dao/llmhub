'use client';

import { useEffect, useState } from 'react';
import { api, type AdminAccount, type AdminUser, type ProviderRow } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

export default function Dashboard() {
  const [accounts, setAccounts] = useState<AdminAccount[]>([]);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [providers, setProviders] = useState<ProviderRow[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.get<{ data: AdminAccount[] }>('/api/admin/pool/accounts?limit=200'),
      api.get<{ data: AdminUser[] }>('/api/admin/users?limit=200'),
      api.get<{ data: ProviderRow[] }>('/api/admin/providers'),
    ])
      .then(([a, u, p]) => {
        setAccounts(a.data ?? []);
        setUsers(u.data ?? []);
        setProviders(p.data ?? []);
      })
      .catch((err) => setError((err as Error).message));
  }, []);

  const active = accounts.filter((a) => a.status === 'active').length;
  const cooldown = accounts.filter((a) => a.status === 'cooldown' || a.status === 'rate_limited').length;
  const banned = accounts.filter((a) => a.status === 'banned' || a.status === 'archived').length;

  return (
    <div className="space-y-6">
      <PageHeader title="运营总览" subtitle="账号池、用户、厂商概况" />

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Stat label="账号总数" value={accounts.length} />
        <Stat label="活跃账号" value={active} accent="emerald" />
        <Stat label="冷却 / 限流" value={cooldown} accent="amber" />
        <Stat label="封禁 / 归档" value={banned} accent="rose" />
        <Stat label="厂商" value={providers.length} />
        <Stat label="用户" value={users.length} />
      </div>

      <div className="rounded-2xl border border-ink-700 bg-ink-800">
        <div className="border-b border-ink-700 px-5 py-3 text-sm font-medium text-white">
          最近账号（前 10）
        </div>
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-5 py-2.5 text-left">ID</th>
              <th className="px-5 py-2.5 text-left">厂商</th>
              <th className="px-5 py-2.5 text-left">tier</th>
              <th className="px-5 py-2.5 text-right">健康</th>
              <th className="px-5 py-2.5 text-left">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {accounts.slice(0, 10).map((a) => (
              <tr key={a.id}>
                <td className="px-5 py-2.5 mono">{a.id}</td>
                <td className="px-5 py-2.5">{a.provider_id}</td>
                <td className="px-5 py-2.5">{a.tier}</td>
                <td className="px-5 py-2.5 text-right">{a.health_score}</td>
                <td className="px-5 py-2.5">{a.status}</td>
              </tr>
            ))}
            {accounts.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-12 text-center text-ink-500">
                  暂无账号 — 去 /pool 添加
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  accent,
}: {
  label: string;
  value: number;
  accent?: 'emerald' | 'amber' | 'rose';
}) {
  const tone =
    accent === 'emerald'
      ? 'text-emerald-400'
      : accent === 'amber'
        ? 'text-amber-400'
        : accent === 'rose'
          ? 'text-rose-400'
          : 'text-white';
  return (
    <div className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
      <div className="text-sm text-ink-500">{label}</div>
      <div className={`mt-2 text-3xl font-semibold tracking-tight ${tone}`}>{value}</div>
    </div>
  );
}
