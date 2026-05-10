'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import {
  api,
  type Credential,
  type Vendor,
  type VendorAccount,
  type VendorProduct,
} from '@/lib/admin-api';
import { fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';

// 凭据管理：v0.2 的 pool.credentials 平表 CRUD。
export default function CredentialsPage() {
  const [rows, setRows] = useState<Credential[]>([]);
  const [vendors, setVendors] = useState<Vendor[]>([]);
  const [products, setProducts] = useState<VendorProduct[]>([]);
  const [accounts, setAccounts] = useState<VendorAccount[]>([]);
  const [filter, setFilter] = useState({
    vendor: '',
    product: '',
    account_id: '',
    status: '',
    q: '',
  });
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.vendor) params.set('vendor', filter.vendor);
      if (filter.product) params.set('product', filter.product);
      if (filter.account_id) params.set('account_id', filter.account_id);
      if (filter.status) params.set('status', filter.status);
      if (filter.q) params.set('q', filter.q);
      params.set('limit', '200');
      const res = await api.get<{ data: Credential[] }>(
        `/api/admin/credentials?${params}`,
      );
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    Promise.all([
      api.get<{ data: Vendor[] }>('/api/admin/catalog/vendors'),
      api.get<{ data: VendorProduct[] }>('/api/admin/catalog/products'),
      api.get<{ data: VendorAccount[] }>('/api/admin/vendor-accounts?limit=500'),
    ])
      .then(([v, p, a]) => {
        setVendors([...(v.data ?? [])].sort((x, y) => x.id.localeCompare(y.id)));
        setProducts([...(p.data ?? [])].sort((x, y) => x.id.localeCompare(y.id)));
        setAccounts(a.data ?? []);
      })
      .catch(() => {});
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function archive(c: Credential) {
    if (!confirm(`归档凭据「${c.name}」? 该凭据下所有服务绑定将停用。`)) return;
    try {
      await api.delete(`/api/admin/credentials/${c.id}`);
      await load();
    } catch (err) {
      alert((err as Error).message);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="凭据管理"
        subtitle='某主账号 × 某业务板块下的"应用"。凭据上挂服务绑定（调度行）。'
        actions={
          <Link
            href="/credentials/new"
            className="rounded-lg border border-ink-200 bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-50"
          >
            + 新增凭据
          </Link>
        }
      />

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-200 bg-white p-4 text-sm">
        <Filter
          label="厂商"
          value={filter.vendor}
          onChange={(v) => setFilter({ ...filter, vendor: v, product: '' })}
          options={[
            { value: '', label: '全部' },
            ...vendors.map((v) => ({ value: v.id, label: v.name })),
          ]}
        />
        <Filter
          label="业务板块"
          value={filter.product}
          onChange={(v) => setFilter({ ...filter, product: v })}
          options={[
            { value: '', label: '全部' },
            ...products
              .filter((p) => !filter.vendor || p.vendor_id === filter.vendor)
              .map((p) => ({ value: p.id, label: p.id })),
          ]}
        />
        <Filter
          label="主账号"
          value={filter.account_id}
          onChange={(v) => setFilter({ ...filter, account_id: v })}
          options={[
            { value: '', label: '全部' },
            ...accounts.map((a) => ({ value: String(a.id), label: a.name })),
          ]}
        />
        <Filter
          label="状态"
          value={filter.status}
          onChange={(v) => setFilter({ ...filter, status: v })}
          options={[
            { value: '', label: '全部' },
            { value: 'active', label: 'active' },
            { value: 'cooldown', label: 'cooldown' },
            { value: 'banned', label: 'banned' },
            { value: 'archived', label: 'archived' },
          ]}
        />
        <label className="flex flex-1 min-w-[180px] flex-col">
          <span className="text-xs text-ink-500">搜索</span>
          <input
            value={filter.q}
            onChange={(e) => setFilter({ ...filter, q: e.target.value })}
            placeholder="凭据名"
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
              <th className="px-4 py-2.5 text-left">业务板块</th>
              <th className="px-4 py-2.5 text-left">主账号</th>
              <th className="px-4 py-2.5 text-left">cred_id</th>
              <th className="px-4 py-2.5 text-left">env</th>
              <th className="px-4 py-2.5 text-right">健康</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-left">创建</th>
              <th></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-5 py-12 text-center text-ink-500">
                  无凭据 — 用上方 + 新增凭据 创建
                </td>
              </tr>
            ) : (
              rows.map((c) => {
                const acct = accounts.find((a) => a.id === c.account_id);
                return (
                  <tr key={c.id}>
                    <td className="px-4 py-2.5">
                      <Link
                        href={`/admin/credentials/${c.id}`}
                        className="hover:underline"
                      >
                        🔑 {c.name}
                      </Link>
                    </td>
                    <td className="px-4 py-2.5 mono text-xs">{c.product_id}</td>
                    <td className="px-4 py-2.5 text-xs">
                      {acct?.name ?? <span className="text-ink-500">acct_{c.account_id}</span>}
                    </td>
                    <td className="px-4 py-2.5 mono text-xs text-ink-500">cred_{c.id}</td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">{c.env}</td>
                    <td className="px-4 py-2.5 text-right">{c.health_score}</td>
                    <td className="px-4 py-2.5">
                      <StatusPill status={c.status} />
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">
                      {fmtDateTime(c.created_at)}
                    </td>
                    <td className="px-4 py-2.5 text-right text-xs whitespace-nowrap">
                      <Link
                        href={`/admin/credentials/${c.id}`}
                        className="text-ink-800 hover:underline"
                      >
                        详情
                      </Link>
                      {c.status !== 'archived' ? (
                        <button
                          onClick={() => archive(c)}
                          className="ml-3 text-rose-700 hover:underline"
                        >
                          归档
                        </button>
                      ) : null}
                    </td>
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
      : status === 'cooldown' || status === 'rate_limited'
        ? 'bg-amber-100 text-amber-700'
        : status === 'banned' || status === 'archived'
          ? 'bg-rose-100 text-rose-700'
          : 'bg-ink-100 text-ink-800';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}

