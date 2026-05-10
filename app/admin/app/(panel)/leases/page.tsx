'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { api, type AdminLease } from '@/lib/admin-api';
import { fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';

// 活 Lease 监控：实时看哪些 SDK 进程持着真上游凭据 + 强制撤销。
//
// 主要使用场景：
// 1. 异常告警：某用户突然有几百个 lease — 多半是 SDK 死循环
// 2. 安全事件：怀疑用户 API key 泄漏，先全量撤销该用户所有 lease
// 3. 凭据轮换：upstream 厂商提示某 key 被限频，撤销绑定该 binding 的全部 lease
//
// 默认只显示 active 且未过期的；过滤器可以放开看历史。
export default function LeasesPage() {
  const [rows, setRows] = useState<AdminLease[]>([]);
  const [filter, setFilter] = useState({
    user_id: '',
    sku_id: '',
    binding_id: '',
    status: '',
    only_active: '1',
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.user_id) params.set('user_id', filter.user_id);
      if (filter.sku_id) params.set('sku_id', filter.sku_id);
      if (filter.binding_id) params.set('binding_id', filter.binding_id);
      if (filter.status) params.set('status', filter.status);
      if (filter.only_active === '1') params.set('active', '1');
      params.set('limit', '200');
      const res = await api.get<{ data: AdminLease[] }>(`/api/admin/leases?${params}`);
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function onRevoke(l: AdminLease) {
    if (l.status !== 'active') return;
    const reason = prompt('撤销原因（可选）', 'admin_revoke');
    if (reason === null) return;
    try {
      const q = new URLSearchParams({ reason: reason || 'admin_revoke' });
      await api.delete(`/api/admin/leases/${l.lease_id}?${q}`);
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="活 Lease"
        subtitle="SDK 进程拿着真上游凭据的发放记录。撤销后该 lease_id 立即失效，SDK 下次调用会重新申请。"
      />

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-200 bg-white p-4 text-sm">
        <FilterInput
          label="user_id"
          value={filter.user_id}
          onChange={(v) => setFilter({ ...filter, user_id: v })}
          placeholder="123"
        />
        <FilterInput
          label="sku_id"
          value={filter.sku_id}
          onChange={(v) => setFilter({ ...filter, sku_id: v })}
          placeholder="deepseek-chat"
        />
        <FilterInput
          label="binding_id"
          value={filter.binding_id}
          onChange={(v) => setFilter({ ...filter, binding_id: v })}
          placeholder="11"
        />
        <FilterSelect
          label="状态"
          value={filter.only_active === '1' ? '__active__' : filter.status || '__all__'}
          onChange={(v) => {
            if (v === '__active__') setFilter({ ...filter, only_active: '1', status: '' });
            else if (v === '__all__') setFilter({ ...filter, only_active: '', status: '' });
            else setFilter({ ...filter, only_active: '', status: v });
          }}
          options={[
            { value: '__active__', label: '活 (active+未过期)' },
            { value: '__all__', label: '全部' },
            { value: 'active', label: 'active (含已过期)' },
            { value: 'revoked', label: 'revoked' },
            { value: 'expired', label: 'expired' },
          ]}
        />
        <button
          onClick={load}
          className="self-end rounded-lg bg-brand-600 px-4 py-2 font-medium text-white hover:bg-brand-700 disabled:opacity-60"
          disabled={loading}
        >
          {loading ? '查询中…' : '查询'}
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
              <th className="px-4 py-2.5 text-left">Lease</th>
              <th className="px-4 py-2.5 text-left">用户</th>
              <th className="px-4 py-2.5 text-left">SKU</th>
              <th className="px-4 py-2.5 text-left">Binding</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-left">签发</th>
              <th className="px-4 py-2.5 text-left">过期</th>
              <th className="px-4 py-2.5 text-right">用量</th>
              <th className="px-4 py-2.5 text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-5 py-12 text-center text-ink-500">
                  无匹配 lease
                </td>
              </tr>
            ) : (
              rows.map((l) => (
                <tr key={l.lease_id}>
                  <td className="px-4 py-2.5">
                    <div className="mono text-[11px] text-ink-500">{l.lease_id.slice(0, 8)}…</div>
                    {l.client_ip ? (
                      <div className="mono text-[10px] text-ink-500">{l.client_ip}</div>
                    ) : null}
                  </td>
                  <td className="px-4 py-2.5 mono text-xs">
                    <Link href={`/admin/users/${l.user_id}`} className="text-brand-600 hover:underline">
                      #{l.user_id}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 mono text-xs">{l.sku_id}</td>
                  <td className="px-4 py-2.5 mono text-xs">
                    <span className="text-ink-500">b{l.binding_id}</span>
                    <span className="ml-1 text-[10px] text-ink-500">/c{l.credential_id}</span>
                  </td>
                  <td className="px-4 py-2.5">
                    <LeaseStatusPill status={l.status} expiresAt={l.expires_at} />
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(l.issued_at)}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(l.expires_at)}</td>
                  <td className="px-4 py-2.5 text-right text-xs">
                    <div>{l.use_count} 次</div>
                    <div className="text-[10px] text-ink-500">
                      {l.total_input_units}↓ / {l.total_output_units}↑
                    </div>
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    {l.status === 'active' ? (
                      <button
                        onClick={() => onRevoke(l)}
                        className="rounded-md border border-rose-300 px-2 py-0.5 text-[11px] text-rose-700 hover:bg-rose-50"
                      >
                        撤销
                      </button>
                    ) : (
                      <span className="text-[11px] text-ink-500">{l.revoke_reason || '—'}</span>
                    )}
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

function FilterInput({
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
        className="mt-1 rounded-lg border border-ink-200 bg-white px-3 py-1.5 text-sm text-ink-900 outline-none focus:border-brand-500"
      />
    </label>
  );
}

function FilterSelect({
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

function LeaseStatusPill({ status, expiresAt }: { status: string; expiresAt: string }) {
  const expired = new Date(expiresAt).getTime() <= Date.now();
  const effective = status === 'active' && !expired ? 'active' : status === 'active' && expired ? 'expired_soft' : status;
  const tone =
    effective === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : effective === 'revoked'
        ? 'bg-rose-100 text-rose-700'
        : 'bg-ink-100 text-ink-500';
  const label = effective === 'expired_soft' ? '已过期' : effective;
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] ${tone}`}>{label}</span>;
}
