'use client';

import { use, useEffect, useState } from 'react';
import {
  api,
  type Capability,
  type CredentialDetail,
  type ServiceBinding,
  type VendorAccount,
  type VendorProduct,
} from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../../_components/page-header';

// 凭据详情页：凭据信息 + 服务绑定（调度行）+ 加服务弹层。
export default function CredentialDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const credID = Number(id);
  const [data, setData] = useState<CredentialDetail | null>(null);
  const [account, setAccount] = useState<VendorAccount | null>(null);
  const [products, setProducts] = useState<VendorProduct[]>([]);
  const [caps, setCaps] = useState<Capability[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);

  async function load() {
    setError(null);
    try {
      const detail = await api.get<CredentialDetail>(`/api/admin/credentials/${credID}`);
      setData(detail);
      // pull supporting dictionaries in parallel
      const [acctRes, prodRes, capRes] = await Promise.all([
        api.get<VendorAccount>(`/api/admin/vendor-accounts/${detail.credential.account_id}`).catch(() => null),
        products.length === 0
          ? api.get<{ data: VendorProduct[] }>('/api/admin/catalog/products')
          : Promise.resolve(null),
        caps.length === 0
          ? api.get<{ data: Capability[] }>('/api/admin/catalog/capabilities')
          : Promise.resolve(null),
      ]);
      if (acctRes) setAccount(acctRes);
      if (prodRes) setProducts(prodRes.data);
      if (capRes) setCaps(capRes.data);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [credID]);

  const product = data ? products.find((p) => p.id === data.credential.product_id) : null;
  const allowedCaps = product?.allowed_capabilities ?? [];
  const boundCaps = new Set((data?.bindings ?? []).map((b) => b.capability));
  const availableCaps = allowedCaps.filter((c) => !boundCaps.has(c));

  if (error) {
    return (
      <div className="space-y-4">
        <PageHeader title="凭据详情" />
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="space-y-4">
        <PageHeader title="凭据详情" />
        <div className="text-sm text-ink-500">加载中…</div>
      </div>
    );
  }

  const c = data.credential;
  return (
    <div className="space-y-6">
      <PageHeader
        title={`🔑 ${c.name}`}
        subtitle={`cred_${c.id} · ${c.product_id}${product ? ` · ${product.name}` : ''}`}
        actions={
          availableCaps.length > 0 ? (
            <button
              onClick={() => setShowAdd((s) => !s)}
              className="rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-ink-900"
            >
              {showAdd ? '收起' : '+ 加服务'}
            </button>
          ) : (
            <span className="text-xs text-ink-500">已绑全部允许的能力</span>
          )
        }
      />

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Info label="厂商" value={c.vendor_id} />
        <Info label="主账号" value={account ? account.name : `acct_${c.account_id}`} />
        <Info label="环境" value={c.env} />
        <Info label="状态" value={c.status} />
        <Info label="健康" value={String(c.health_score)} />
        <Info label="创建时间" value={fmtDateTime(c.created_at)} />
      </div>

      {showAdd && availableCaps.length > 0 ? (
        <AddBindingForm
          credID={credID}
          available={availableCaps}
          caps={caps}
          onClose={() => setShowAdd(false)}
          onAdded={() => {
            setShowAdd(false);
            load();
          }}
        />
      ) : null}

      <div className="rounded-2xl border border-ink-700 bg-ink-800 overflow-hidden">
        <div className="px-5 py-3 border-b border-ink-700 text-sm font-medium">
          服务绑定（{data.bindings.length}）
        </div>
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2 text-left">能力</th>
              <th className="px-4 py-2 text-left">tier</th>
              <th className="px-4 py-2 text-right">QPS</th>
              <th className="px-4 py-2 text-right">日用 / 限额</th>
              <th className="px-4 py-2 text-right">cost_basis</th>
              <th className="px-4 py-2 text-right">健康</th>
              <th className="px-4 py-2 text-left">状态</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {data.bindings.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-8 text-center text-ink-500">
                  无服务绑定
                </td>
              </tr>
            ) : (
              data.bindings.map((b) => <BindingRow key={b.id} b={b} />)
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-ink-700 bg-ink-800 p-3">
      <div className="text-[11px] text-ink-500">{label}</div>
      <div className="mt-0.5 text-sm">{value}</div>
    </div>
  );
}

function BindingRow({ b }: { b: ServiceBinding }) {
  return (
    <tr>
      <td className="px-4 py-2.5 mono">{b.capability}</td>
      <td className="px-4 py-2.5">{b.tier}</td>
      <td className="px-4 py-2.5 text-right">{b.qps_limit ?? '—'}</td>
      <td className="px-4 py-2.5 text-right text-xs">
        {fmtCents(b.daily_used_cents)} /{' '}
        {b.daily_limit_cents != null ? fmtCents(b.daily_limit_cents) : '—'}
      </td>
      <td className="px-4 py-2.5 text-right text-xs">{b.cost_basis_cents}</td>
      <td className="px-4 py-2.5 text-right">{b.health_score}</td>
      <td className="px-4 py-2.5">
        <span
          className={`inline-flex rounded-full px-2 py-0.5 text-xs ${
            b.status === 'active'
              ? 'bg-emerald-500/20 text-emerald-300'
              : b.status === 'cooldown'
                ? 'bg-amber-500/20 text-amber-300'
                : 'bg-rose-500/20 text-rose-300'
          }`}
        >
          {b.status}
        </span>
      </td>
    </tr>
  );
}

function AddBindingForm({
  credID,
  available,
  caps,
  onClose,
  onAdded,
}: {
  credID: number;
  available: string[];
  caps: Capability[];
  onClose: () => void;
  onAdded: () => void;
}) {
  const [capability, setCapability] = useState(available[0] ?? '');
  const [tier, setTier] = useState('pro');
  const [qps, setQPS] = useState<number>(10);
  const [daily, setDaily] = useState<number>(20000);
  const [cost, setCost] = useState<number>(1.0);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.post(`/api/admin/credentials/${credID}/bindings`, {
        capability,
        tier,
        qps_limit: qps || null,
        daily_limit_cents: daily || null,
        cost_basis_cents: Number(cost),
      });
      onAdded();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form
      onSubmit={onSubmit}
      className="rounded-2xl border border-ink-700 bg-ink-800 p-5"
    >
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-semibold text-white">加服务</div>
        <button
          type="button"
          onClick={onClose}
          className="text-xs text-ink-500 hover:text-ink-200"
        >
          关闭
        </button>
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <label className="block">
          <span className="text-xs text-ink-500">能力</span>
          <select
            value={capability}
            onChange={(e) => setCapability(e.target.value)}
            className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm"
          >
            {available.map((c) => {
              const cap = caps.find((x) => x.id === c);
              return (
                <option key={c} value={c}>
                  {c} {cap ? `· ${cap.display_name}` : ''}
                </option>
              );
            })}
          </select>
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">tier</span>
          <select
            value={tier}
            onChange={(e) => setTier(e.target.value)}
            className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm"
          >
            <option value="free">free</option>
            <option value="pro">pro</option>
            <option value="enterprise">enterprise</option>
          </select>
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">QPS</span>
          <input
            type="number"
            value={qps}
            onChange={(e) => setQPS(Number(e.target.value))}
            className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">日限额 (cents)</span>
          <input
            type="number"
            value={daily}
            onChange={(e) => setDaily(Number(e.target.value))}
            className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">cost_basis (cents)</span>
          <input
            type="number"
            step="0.01"
            value={cost}
            onChange={(e) => setCost(Number(e.target.value))}
            className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm"
          />
        </label>
      </div>
      {error ? <div className="mt-3 text-sm text-rose-400">{error}</div> : null}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-lg border border-ink-700 px-4 py-2 text-sm text-ink-200"
        >
          取消
        </button>
        <button
          type="submit"
          disabled={submitting}
          className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 disabled:opacity-60"
        >
          {submitting ? '提交中…' : '加服务'}
        </button>
      </div>
    </form>
  );
}
