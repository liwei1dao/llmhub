'use client';

import { useEffect, useState } from 'react';
import { api, type AdminAccount } from '@/lib/api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';
import CreateAccountForm from './_components/create-form';

export default function PoolPage() {
  const [rows, setRows] = useState<AdminAccount[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState({ provider: '', tier: '', status: '' });
  const [showForm, setShowForm] = useState(false);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.provider) params.set('provider', filter.provider);
      if (filter.tier) params.set('tier', filter.tier);
      if (filter.status) params.set('status', filter.status);
      params.set('limit', '200');
      const res = await api.get<{ data: AdminAccount[] }>(`/api/admin/pool/accounts?${params}`);
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []); // initial load only; filter button reloads explicitly

  async function archive(id: number) {
    if (!confirm(`归档账号 #${id}? 调度将不再使用此账号。`)) return;
    try {
      await api.delete(`/api/admin/pool/accounts/${id}`);
      await load();
    } catch (err) {
      alert((err as Error).message);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="账号池"
        subtitle="上游厂商账号的录入与状态机"
        actions={
          <button
            onClick={() => setShowForm(true)}
            className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200"
          >
            + 新增账号
          </button>
        }
      />

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4 text-sm">
        <Filter label="厂商" value={filter.provider} onChange={(v) => setFilter({ ...filter, provider: v })} placeholder="volc / deepseek / ..." />
        <Filter label="Tier" value={filter.tier} onChange={(v) => setFilter({ ...filter, tier: v })} placeholder="free / pro / ..." />
        <Filter label="状态" value={filter.status} onChange={(v) => setFilter({ ...filter, status: v })} placeholder="active / cooldown / banned" />
        <button
          onClick={load}
          className="self-end rounded-lg bg-brand-600 px-4 py-2 font-medium text-white hover:bg-brand-700"
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      {showForm ? (
        <CreateAccountForm
          onClose={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false);
            load();
          }}
        />
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-700 bg-ink-800">
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">ID</th>
              <th className="px-4 py-2.5 text-left">厂商</th>
              <th className="px-4 py-2.5 text-left">Tier</th>
              <th className="px-4 py-2.5 text-left">来源</th>
              <th className="px-4 py-2.5 text-left">能力</th>
              <th className="px-4 py-2.5 text-right">健康</th>
              <th className="px-4 py-2.5 text-right">日用 / 限额</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-left">创建</th>
              <th className="px-4 py-2.5"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={10} className="px-5 py-12 text-center text-ink-500">
                  无账号
                </td>
              </tr>
            ) : (
              rows.map((a) => (
                <tr key={a.id}>
                  <td className="px-4 py-2.5 mono">{a.id}</td>
                  <td className="px-4 py-2.5">{a.provider_id}</td>
                  <td className="px-4 py-2.5">{a.tier}</td>
                  <td className="px-4 py-2.5 text-ink-500">{a.origin}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">
                    {(a.supported_capabilities ?? []).join(', ') || '-'}
                  </td>
                  <td className="px-4 py-2.5 text-right">{a.health_score}</td>
                  <td className="px-4 py-2.5 text-right text-xs">
                    {fmtCents(a.daily_used_cents)} / {fmtCents(a.daily_limit_cents)}
                  </td>
                  <td className="px-4 py-2.5">
                    <StatusPill status={a.status} />
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(a.created_at)}</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => archive(a.id)}
                      className="text-xs text-rose-400 hover:underline"
                    >
                      归档
                    </button>
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

function Filter({
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
    <label className="flex flex-col">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-500/20 text-emerald-300'
      : status === 'cooldown' || status === 'rate_limited'
        ? 'bg-amber-500/20 text-amber-300'
        : status === 'banned' || status === 'archived'
          ? 'bg-rose-500/20 text-rose-300'
          : 'bg-ink-700 text-ink-200';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}
