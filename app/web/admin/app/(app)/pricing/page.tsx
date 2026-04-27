'use client';

import { useEffect, useState } from 'react';
import { api, type PricingRow } from '@/lib/api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';

export default function PricingPage() {
  const [rows, setRows] = useState<PricingRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [model, setModel] = useState('');
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (model) params.set('model_id', model);
      params.set('limit', '500');
      const res = await api.get<{ data: PricingRow[] }>(`/api/admin/pricing?${params}`);
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
      <PageHeader
        title="定价"
        subtitle="按模型 / 能力域显示零售价与成本基价（catalog.pricing）"
      />

      <div className="flex gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4">
        <input
          value={model}
          onChange={(e) => setModel(e.target.value)}
          placeholder="model_id (可选)"
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
              <th className="px-4 py-2.5 text-left">模型</th>
              <th className="px-4 py-2.5 text-left">能力</th>
              <th className="px-4 py-2.5 text-left">单位</th>
              <th className="px-4 py-2.5 text-right">零售价 / 单位</th>
              <th className="px-4 py-2.5 text-right">成本 / 单位</th>
              <th className="px-4 py-2.5 text-right">毛利</th>
              <th className="px-4 py-2.5 text-left">生效</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-12 text-center text-ink-500">
                  无定价
                </td>
              </tr>
            ) : (
              rows.map((p, i) => {
                const margin = p.retail_cents_per_unit - p.cost_cents_per_unit;
                const marginPct = p.retail_cents_per_unit > 0 ? Math.round((margin / p.retail_cents_per_unit) * 100) : 0;
                return (
                  <tr key={`${p.model_id}-${p.capability_id}-${i}`}>
                    <td className="px-4 py-2.5 mono">{p.model_id}</td>
                    <td className="px-4 py-2.5">{p.capability_id}</td>
                    <td className="px-4 py-2.5">{p.unit}</td>
                    <td className="px-4 py-2.5 text-right">{fmtCents(p.retail_cents_per_unit)}</td>
                    <td className="px-4 py-2.5 text-right text-ink-500">{fmtCents(p.cost_cents_per_unit)}</td>
                    <td className={`px-4 py-2.5 text-right ${margin > 0 ? 'text-emerald-300' : 'text-rose-300'}`}>
                      {fmtCents(margin)} ({marginPct}%)
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(p.effective_at)}</td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
