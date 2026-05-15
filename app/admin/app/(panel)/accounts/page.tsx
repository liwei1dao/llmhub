'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { api, type Vendor, type VendorAccount } from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';
import CreateMasterForm from './_create-form';

// 主账号管理：v0.2 的 pool.vendor_accounts 表。
// 主账号承担余额查询/对账，不调业务 API（业务凭据见 /admin/credentials）。
export default function AccountsPage() {
  const router = useRouter();
  const [rows, setRows] = useState<VendorAccount[]>([]);
  const [vendors, setVendors] = useState<Vendor[]>([]);
  const [filter, setFilter] = useState({ vendor: '', status: '', q: '' });
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  async function load() {
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.vendor) params.set('vendor', filter.vendor);
      if (filter.status) params.set('status', filter.status);
      if (filter.q) params.set('q', filter.q);
      params.set('limit', '200');
      const res = await api.get<{ data: VendorAccount[] }>(
        `/api/admin/vendor-accounts?${params}`,
      );
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    api
      .get<{ data: Vendor[] }>('/api/admin/catalog/vendors')
      .then((r) => setVendors([...(r.data ?? [])].sort((a, b) => a.id.localeCompare(b.id))))
      .catch(() => {});
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function archive(id: number, name: string) {
    if (!confirm(`归档主账号「${name}」? 该账号下所有凭据将停用。`)) return;
    try {
      await api.delete(`/api/admin/vendor-accounts/${id}`);
      await load();
    } catch (err) {
      alert((err as Error).message);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="主账号管理"
        subtitle="厂商主账号 = 行政/结算单元。承担余额查询和对账。"
        actions={
          <button
            onClick={() => setShowForm((s) => !s)}
            className="rounded-lg border border-ink-200 bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-50"
          >
            {showForm ? '收起' : '+ 新增主账号'}
          </button>
        }
      />

      {showForm ? (
        <CreateMasterForm
          vendors={vendors}
          onClose={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false);
            load();
          }}
        />
      ) : null}

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-200 bg-white p-4 text-sm">
        <Filter
          label="厂商"
          value={filter.vendor}
          onChange={(v) => setFilter({ ...filter, vendor: v })}
          options={[
            { value: '', label: '全部' },
            ...vendors.map((v) => ({ value: v.id, label: v.name })),
          ]}
        />
        <Filter
          label="状态"
          value={filter.status}
          onChange={(v) => setFilter({ ...filter, status: v })}
          options={[
            { value: '', label: '全部' },
            { value: 'active', label: 'active' },
            { value: 'frozen', label: 'frozen' },
            { value: 'archived', label: 'archived' },
          ]}
        />
        <label className="flex flex-1 min-w-[200px] flex-col">
          <span className="text-xs text-ink-500">搜索</span>
          <input
            value={filter.q}
            onChange={(e) => setFilter({ ...filter, q: e.target.value })}
            placeholder="名称 / 主体"
            className="mt-1 rounded-lg border border-ink-200 bg-white px-3 py-1.5 text-sm text-ink-900 outline-none focus:border-brand-500"
          />
        </label>
        <button
          onClick={load}
          className="self-end rounded-lg bg-brand-600 px-4 py-2 font-medium text-white hover:bg-brand-700"
        >
          查询
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-200 bg-white">
        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">名称</th>
              <th className="px-4 py-2.5 text-left">厂商</th>
              <th className="px-4 py-2.5 text-left">acct_id</th>
              <th className="px-4 py-2.5 text-left">主体</th>
              <th className="px-4 py-2.5 text-right">余额</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-left">创建</th>
              <th></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={8} className="px-5 py-12 text-center text-ink-500">
                  无主账号 — 用上方 + 新增主账号 创建
                </td>
              </tr>
            ) : (
              rows.map((a) => (
                <tr
                  key={a.id}
                  onClick={() => router.push(`/accounts/${a.id}`)}
                  className="cursor-pointer hover:bg-ink-50"
                >
                  <td className="px-4 py-2.5 text-ink-800">🪪 {a.name}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{a.vendor_id}</td>
                  <td className="px-4 py-2.5 mono text-xs text-ink-500">acct_{a.id}</td>
                  <td className="px-4 py-2.5 text-xs">{a.entity ?? '—'}</td>
                  <td className="px-4 py-2.5 text-right text-emerald-700">
                    {a.last_balance_cents != null
                      ? fmtCents(a.last_balance_cents)
                      : <span className="text-ink-500">—</span>}
                  </td>
                  <td className="px-4 py-2.5">
                    <StatusPill status={a.status} />
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(a.created_at)}</td>
                  <td className="px-4 py-2.5 text-right text-xs whitespace-nowrap">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        archive(a.id, a.name);
                      }}
                      className="text-rose-700 hover:underline"
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
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="flex flex-col">
      <span className="text-xs text-ink-500">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 rounded-lg border border-ink-200 bg-white px-3 py-1.5 text-sm text-ink-900 outline-none focus:border-brand-500"
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

function StatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'frozen'
        ? 'bg-amber-100 text-amber-700'
        : 'bg-rose-100 text-rose-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}
