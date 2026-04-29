'use client';

import { useEffect, useState } from 'react';
import { api, type AdminUser } from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';

export default function UsersPage() {
  const [rows, setRows] = useState<AdminUser[]>([]);
  const [q, setQ] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (q) params.set('q', q);
      params.set('limit', '200');
      const res = await api.get<{ data: AdminUser[] }>(`/api/admin/users?${params}`);
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader title="用户" subtitle="按邮箱 / 手机号搜索" />

      <div className="flex gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4">
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="邮箱 / 手机号"
          className="flex-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
        />
        <button
          onClick={load}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-700 bg-ink-800">
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">ID</th>
              <th className="px-4 py-2.5 text-left">邮箱</th>
              <th className="px-4 py-2.5 text-left">手机</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-right">风险分</th>
              <th className="px-4 py-2.5 text-right">QPS</th>
              <th className="px-4 py-2.5 text-right">日消费上限</th>
              <th className="px-4 py-2.5 text-left">注册</th>
              <th className="px-4 py-2.5 text-left">最近登录</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-5 py-12 text-center text-ink-500">
                  无用户
                </td>
              </tr>
            ) : (
              rows.map((u) => (
                <tr key={u.id}>
                  <td className="px-4 py-2.5 mono">{u.id}</td>
                  <td className="px-4 py-2.5">{u.email ?? '-'}</td>
                  <td className="px-4 py-2.5">{u.phone ?? '-'}</td>
                  <td className="px-4 py-2.5">{u.status}</td>
                  <td className="px-4 py-2.5 text-right">{u.risk_score}</td>
                  <td className="px-4 py-2.5 text-right">{u.qps_limit}</td>
                  <td className="px-4 py-2.5 text-right">{fmtCents(u.daily_spend_limit_cents)}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(u.created_at)}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(u.last_login_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
