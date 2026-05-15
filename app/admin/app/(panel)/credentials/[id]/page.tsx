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
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
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
              className="rounded-lg border border-ink-200 bg-white px-3 py-1.5 text-sm font-medium text-ink-900 hover:bg-ink-50"
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

      {product ? <RotateAuthPayload credID={credID} product={product} /> : null}

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

      <div className="rounded-2xl border border-ink-200 bg-white overflow-hidden">
        <div className="px-5 py-3 border-b border-ink-200 text-sm font-medium">
          服务绑定（{data.bindings.length}）
        </div>
        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
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
          <tbody className="divide-y divide-ink-200">
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
    <div className="rounded-lg border border-ink-200 bg-white p-3">
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
              ? 'bg-emerald-100 text-emerald-700'
              : b.status === 'cooldown'
                ? 'bg-amber-100 text-amber-700'
                : 'bg-rose-100 text-rose-700'
          }`}
        >
          {b.status}
        </span>
      </td>
    </tr>
  );
}

// RotateAuthPayload 用于「轮换上游 api_key」或修复旧凭据（v0 创建时
// 没真正把 auth_payload 写进 vault，状态是 devcred://…）。提交即 PATCH
// /api/admin/credentials/{id}/auth-payload，后端用 devjson:// 编码进 DB，
// SDK / 控制台测试随后立刻能 resolve 出来。
function RotateAuthPayload({ credID, product }: { credID: number; product: VendorProduct }) {
  const [open, setOpen] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [ok, setOk] = useState(false);

  function setField(k: string, v: string) {
    setValues((cur) => ({ ...cur, [k]: v }));
  }

  async function submit() {
    setErr(null);
    setOk(false);
    // 把空值挤掉，避免覆写时把后端 schema 当成"全空"
    const payload: Record<string, string> = {};
    for (const f of product.credential_schema) {
      const v = (values[f.key] ?? '').trim();
      if (f.required && !v) {
        setErr(`${f.label}（${f.key}）是必填项。`);
        return;
      }
      if (v) payload[f.key] = v;
    }
    if (Object.keys(payload).length === 0) {
      setErr('至少填一个字段。');
      return;
    }
    setBusy(true);
    try {
      await api.patch(`/api/admin/credentials/${credID}/auth-payload`, { auth_payload: payload });
      setValues({});
      setOk(true);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <div className="flex items-center justify-between rounded-2xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm">
        <div>
          <div className="font-medium text-amber-900">更新 / 轮换 auth_payload</div>
          <div className="text-[11px] text-amber-800">
            上游 API Key 滚动 / 修复 v0 占位凭据时点这里。保存后 SDK 立即可用。
          </div>
        </div>
        <button
          onClick={() => setOpen(true)}
          className="rounded-lg border border-amber-400 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 hover:bg-amber-100"
        >
          展开
        </button>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-amber-300 bg-amber-50 p-4">
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-medium text-amber-900">更新 / 轮换 auth_payload</div>
        <button
          onClick={() => setOpen(false)}
          className="text-xs text-amber-800 hover:underline"
        >
          收起
        </button>
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {product.credential_schema.map((f) => (
          <label key={f.key} className="block">
            <span className="text-[11px] text-amber-900">
              {f.label} <span className="mono text-amber-700">{f.key}</span>
              {f.required ? <span className="text-rose-700"> *</span> : null}
            </span>
            <input
              type={f.sensitive ? 'password' : 'text'}
              value={values[f.key] ?? ''}
              onChange={(e) => setField(f.key, e.target.value)}
              placeholder={f.sensitive ? '••••• (粘贴新值)' : ''}
              className="mono mt-1 w-full rounded-lg border border-amber-300 bg-white px-2 py-1.5 text-xs focus:border-amber-500 focus:outline-none"
            />
          </label>
        ))}
      </div>
      {err ? <div className="mt-3 text-xs text-rose-700">{err}</div> : null}
      {ok ? <div className="mt-3 text-xs text-emerald-700">已更新 auth_payload。</div> : null}
      <div className="mt-3 flex items-center gap-2">
        <button
          onClick={submit}
          disabled={busy}
          className="rounded-lg bg-amber-700 px-3 py-1.5 text-xs font-medium text-white hover:bg-amber-800 disabled:opacity-60"
        >
          {busy ? '保存中…' : '保存'}
        </button>
        <span className="text-[11px] text-amber-800">
          这一步会立刻替换 DB 里的 auth_payload_ref；不影响 bindings / 健康分。
        </span>
      </div>
    </div>
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
      className="rounded-2xl border border-ink-200 bg-white p-5"
    >
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-semibold text-ink-900">加服务</div>
        <button
          type="button"
          onClick={onClose}
          className="text-xs text-ink-500 hover:text-ink-800"
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
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm"
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
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm"
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
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">日限额 (cents)</span>
          <input
            type="number"
            value={daily}
            onChange={(e) => setDaily(Number(e.target.value))}
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm"
          />
        </label>
        <label className="block">
          <span className="text-xs text-ink-500">cost_basis (cents)</span>
          <input
            type="number"
            step="0.01"
            value={cost}
            onChange={(e) => setCost(Number(e.target.value))}
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm"
          />
        </label>
      </div>
      {error ? <div className="mt-3 text-sm text-rose-700">{error}</div> : null}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-lg border border-ink-200 px-4 py-2 text-sm text-ink-800"
        >
          取消
        </button>
        <button
          type="submit"
          disabled={submitting}
          className="rounded-lg border border-ink-200 bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-50 disabled:opacity-60"
        >
          {submitting ? '提交中…' : '加服务'}
        </button>
      </div>
    </form>
  );
}
