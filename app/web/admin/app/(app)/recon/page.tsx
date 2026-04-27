'use client';

import { useEffect, useState } from 'react';
import { api, type ReconRow } from '@/lib/api';
import { fmtCents } from '@/lib/format';
import PageHeader from '../_components/page-header';

export default function ReconPage() {
  const [day, setDay] = useState(() => new Date().toISOString().slice(0, 10));
  const [provider, setProvider] = useState('');
  const [rows, setRows] = useState<ReconRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (day) params.set('day', day);
      if (provider) params.set('provider', provider);
      params.set('limit', '500');
      const res = await api.get<{ data: ReconRow[] }>(`/api/admin/reconciliation?${params}`);
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

  const summary = rows.reduce(
    (acc, r) => ({
      upstream: acc.upstream + r.upstream_cents,
      metered: acc.metered + r.metered_cents,
      delta: acc.delta + r.delta_cents,
    }),
    { upstream: 0, metered: 0, delta: 0 },
  );

  return (
    <div className="space-y-6">
      <PageHeader title="对账" subtitle="按天比对上游账单与本平台计量值" />

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4">
        <label className="flex flex-col">
          <span className="text-xs text-ink-500">日期</span>
          <input
            type="date"
            value={day}
            onChange={(e) => setDay(e.target.value)}
            className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
          />
        </label>
        <label className="flex flex-col">
          <span className="text-xs text-ink-500">厂商（可选）</span>
          <input
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            placeholder="volc / deepseek / ..."
            className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
          />
        </label>
        <button
          onClick={load}
          className="self-end rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700"
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Stat label="上游账单合计" value={fmtCents(summary.upstream)} />
        <Stat label="平台计量合计" value={fmtCents(summary.metered)} />
        <Stat
          label="差额"
          value={fmtCents(summary.delta)}
          tone={summary.delta === 0 ? 'emerald' : 'rose'}
        />
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
              <th className="px-4 py-2.5 text-left">日期</th>
              <th className="px-4 py-2.5 text-left">厂商</th>
              <th className="px-4 py-2.5 text-left">账号</th>
              <th className="px-4 py-2.5 text-right">上游</th>
              <th className="px-4 py-2.5 text-right">平台</th>
              <th className="px-4 py-2.5 text-right">差额</th>
              <th className="px-4 py-2.5 text-left">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-12 text-center text-ink-500">
                  无对账记录
                </td>
              </tr>
            ) : (
              rows.map((r) => (
                <tr key={`${r.day}-${r.account_id}`}>
                  <td className="px-4 py-2.5 mono">{r.day}</td>
                  <td className="px-4 py-2.5">{r.provider_id}</td>
                  <td className="px-4 py-2.5 mono">{r.account_id}</td>
                  <td className="px-4 py-2.5 text-right">{fmtCents(r.upstream_cents)}</td>
                  <td className="px-4 py-2.5 text-right">{fmtCents(r.metered_cents)}</td>
                  <td className={`px-4 py-2.5 text-right ${r.delta_cents === 0 ? 'text-emerald-300' : 'text-rose-300'}`}>
                    {fmtCents(r.delta_cents)}
                  </td>
                  <td className="px-4 py-2.5">{r.status}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: 'emerald' | 'rose' }) {
  const cls =
    tone === 'emerald' ? 'text-emerald-400' : tone === 'rose' ? 'text-rose-400' : 'text-white';
  return (
    <div className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
      <div className="text-sm text-ink-500">{label}</div>
      <div className={`mt-2 text-3xl font-semibold tracking-tight ${cls}`}>{value}</div>
    </div>
  );
}
