'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import {
  api,
  type Capability,
  type CreateCredentialReq,
  type Vendor,
  type VendorAccount,
  type VendorProduct,
} from '@/lib/admin-api';
import PageHeader from '../../_components/page-header';

// 5 步向导：选主账号 → 选板块 → 填凭据 → 选服务+配额 → 确认。
//
// 关键约束（与服务端校验对齐）：
//   - 选板块时只列与所选主账号同 vendor 的板块
//   - 选服务时只列该板块允许的能力
//   - 提交时单事务创建 1 凭据 + N binding
type Step = 1 | 2 | 3 | 4 | 5;

type BindingDraft = {
  capability: string;
  selected: boolean;
  tier: string;
  qps_limit: number;
  daily_limit_cents: number;
  cost_basis_cents: number;
};

export default function NewCredentialWizardPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>(1);

  // dictionaries (loaded once)
  const [vendors, setVendors] = useState<Vendor[]>([]);
  const [products, setProducts] = useState<VendorProduct[]>([]);
  const [caps, setCaps] = useState<Capability[]>([]);
  const [accounts, setAccounts] = useState<VendorAccount[]>([]);
  const [loadErr, setLoadErr] = useState<string | null>(null);

  // wizard state
  const [accountID, setAccountID] = useState<number | null>(null);
  const [productID, setProductID] = useState<string>('');
  const [name, setName] = useState('');
  // 凭据的 env 字段 DB 必填且默认 production；当前阶段不暴露给运营选择。
  const env = 'production';
  const [authPayload, setAuthPayload] = useState<Record<string, string>>({});
  const [bindings, setBindings] = useState<BindingDraft[]>([]);

  const [submitErr, setSubmitErr] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    Promise.all([
      api.get<{ data: Vendor[] }>('/api/admin/catalog/vendors'),
      api.get<{ data: VendorProduct[] }>('/api/admin/catalog/products'),
      api.get<{ data: Capability[] }>('/api/admin/catalog/capabilities'),
      api.get<{ data: VendorAccount[] }>('/api/admin/vendor-accounts?limit=500&status=active'),
    ])
      .then(([v, p, c, a]) => {
        setVendors(v.data);
        setProducts(p.data);
        setCaps(c.data);
        setAccounts(a.data);
      })
      .catch((err) => setLoadErr((err as Error).message));
  }, []);

  const account = useMemo(
    () => accounts.find((a) => a.id === accountID) ?? null,
    [accounts, accountID],
  );
  const vendor = useMemo(
    () => vendors.find((v) => v.id === account?.vendor_id) ?? null,
    [vendors, account],
  );
  const product = useMemo(
    () => products.find((p) => p.id === productID) ?? null,
    [products, productID],
  );

  // when product changes, reset bindings drafts to its allowed capabilities
  useEffect(() => {
    if (!product) {
      setBindings([]);
      return;
    }
    setBindings(
      product.allowed_capabilities.map((capID) => ({
        capability: capID,
        selected: false,
        tier: 'pro',
        qps_limit: 10,
        daily_limit_cents: 20000,
        cost_basis_cents: 1.0,
      })),
    );
  }, [product]);

  function nextable(): boolean {
    if (step === 1) return accountID != null;
    if (step === 2) return !!product;
    if (step === 3) {
      if (!name.trim()) return false;
      const req = product?.credential_schema.filter((f) => f.required) ?? [];
      return req.every((f) => (authPayload[f.key] ?? '').trim().length > 0);
    }
    if (step === 4) return bindings.some((b) => b.selected);
    return true;
  }

  async function submit() {
    if (!product || !accountID) return;
    setSubmitting(true);
    setSubmitErr(null);
    try {
      const body: CreateCredentialReq = {
        account_id: accountID,
        product_id: product.id,
        name,
        env,
        auth_payload: authPayload,
        bindings: bindings
          .filter((b) => b.selected)
          .map((b) => ({
            capability: b.capability,
            tier: b.tier,
            qps_limit: b.qps_limit || null,
            daily_limit_cents: b.daily_limit_cents || null,
            cost_basis_cents: b.cost_basis_cents,
          })),
      };
      await api.post('/api/admin/credentials', body);
      router.push('/credentials');
    } catch (err) {
      setSubmitErr((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="新增凭据"
        subtitle="5 步：选主账号 → 选板块 → 填凭据 → 选服务+配额 → 确认。"
      />

      {loadErr ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {loadErr}
        </div>
      ) : null}

      <ol className="flex items-center gap-2">
        {[
          { n: 1, label: '① 选主账号' },
          { n: 2, label: '② 选板块' },
          { n: 3, label: '③ 填凭据' },
          { n: 4, label: '④ 选服务+配额' },
          { n: 5, label: '⑤ 确认' },
        ].map((s, i, arr) => (
          <div key={s.n} className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => s.n <= step && setStep(s.n as Step)}
              className={`rounded-full text-xs px-3 py-1.5 ${
                s.n === step
                  ? 'bg-brand-600 text-white'
                  : s.n < step
                    ? 'bg-ink-100 text-ink-800 cursor-pointer hover:bg-ink-100'
                    : 'bg-ink-100 text-ink-500'
              }`}
            >
              {s.label}
            </button>
            {i < arr.length - 1 ? <div className="flex-1 border-t border-dashed border-ink-200 w-12" /> : null}
          </div>
        ))}
      </ol>

      {/* Step 1: pick account */}
      {step === 1 ? (
        <Step1
          accounts={accounts}
          vendors={vendors}
          accountID={accountID}
          onPick={setAccountID}
        />
      ) : null}

      {/* Step 2: pick product (filtered by account vendor) */}
      {step === 2 && vendor ? (
        <Step2
          vendor={vendor}
          products={products.filter((p) => p.vendor_id === vendor.id)}
          productID={productID}
          onPick={setProductID}
        />
      ) : null}

      {/* Step 3: credential payload */}
      {step === 3 && product ? (
        <Step3
          product={product}
          name={name}
          authPayload={authPayload}
          onChangeName={setName}
          onChangeField={(k, v) => setAuthPayload((p) => ({ ...p, [k]: v }))}
        />
      ) : null}

      {/* Step 4: pick services + per-binding quota */}
      {step === 4 && product ? (
        <Step4 caps={caps} bindings={bindings} setBindings={setBindings} />
      ) : null}

      {/* Step 5: confirm */}
      {step === 5 && product && account ? (
        <Step5
          account={account}
          product={product}
          name={name}
          bindings={bindings.filter((b) => b.selected)}
        />
      ) : null}

      {submitErr ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {submitErr}
        </div>
      ) : null}

      <div className="flex justify-between pt-4 border-t border-ink-200">
        <button
          type="button"
          onClick={() => setStep((s) => (s > 1 ? ((s - 1) as Step) : s))}
          disabled={step === 1}
          className="rounded-lg border border-ink-200 px-4 py-2 text-sm text-ink-800 disabled:opacity-50"
        >
          ← 上一步
        </button>
        {step < 5 ? (
          <button
            type="button"
            onClick={() => setStep((s) => (s + 1) as Step)}
            disabled={!nextable()}
            className="rounded-lg border border-ink-200 bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-50 disabled:opacity-50"
          >
            下一步 →
          </button>
        ) : (
          <button
            type="button"
            onClick={submit}
            disabled={submitting || !nextable()}
            className="rounded-lg bg-brand-600 px-5 py-2 text-sm font-medium text-white disabled:opacity-50"
          >
            {submitting
              ? '提交中…'
              : `提交（创建 1 凭据 + ${bindings.filter((b) => b.selected).length} 个服务绑定）`}
          </button>
        )}
      </div>
    </div>
  );
}

// ─── Step 1 ─────────────────────────────────────────────────
function Step1({
  accounts,
  vendors,
  accountID,
  onPick,
}: {
  accounts: VendorAccount[];
  vendors: Vendor[];
  accountID: number | null;
  onPick: (id: number) => void;
}) {
  const byVendor: Record<string, VendorAccount[]> = {};
  for (const a of accounts) (byVendor[a.vendor_id] ??= []).push(a);

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5 space-y-4">
      <div className="text-sm text-ink-500">
        先选主账号——主账号的厂商决定了下一步能选哪些板块。
      </div>
      {Object.keys(byVendor).length === 0 ? (
        <div className="rounded-lg border border-dashed border-ink-200 bg-white p-4 text-sm text-ink-500">
          没有可用主账号。请先到「主账号管理」创建一个。
        </div>
      ) : null}
      {vendors
        .filter((v) => byVendor[v.id]?.length > 0)
        .map((v) => (
          <div key={v.id}>
            <div className="text-xs text-ink-500 mb-2">
              {v.name}（{v.id}）
            </div>
            <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
              {byVendor[v.id].map((a) => (
                <label
                  key={a.id}
                  className={`cursor-pointer rounded-lg border p-3 ${
                    a.id === accountID
                      ? 'border-brand-500 ring-2 ring-brand-500/30 bg-white'
                      : 'border-ink-200 bg-white hover:border-ink-500'
                  }`}
                >
                  <input
                    type="radio"
                    name="account"
                    checked={a.id === accountID}
                    onChange={() => onPick(a.id)}
                    className="hidden"
                  />
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-sm font-medium">🪪 {a.name}</div>
                      <div className="mono text-[11px] text-ink-500">acct_{a.id}</div>
                    </div>
                    <span className="text-[11px] text-ink-500">{a.entity ?? ''}</span>
                  </div>
                </label>
              ))}
            </div>
          </div>
        ))}
    </div>
  );
}

// ─── Step 2 ─────────────────────────────────────────────────
function Step2({
  vendor,
  products,
  productID,
  onPick,
}: {
  vendor: Vendor;
  products: VendorProduct[];
  productID: string;
  onPick: (id: string) => void;
}) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="text-sm text-ink-500 mb-3">
        已选主账号厂商 <span className="mono text-brand-200">{vendor.id}</span> · 仅展示该厂商的业务板块。
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        {products.map((p) => (
          <label
            key={p.id}
            className={`cursor-pointer rounded-lg border p-4 ${
              p.id === productID
                ? 'border-brand-500 ring-2 ring-brand-500/30 bg-white'
                : 'border-ink-200 bg-white hover:border-ink-500'
            }`}
          >
            <input
              type="radio"
              name="product"
              checked={p.id === productID}
              onChange={() => onPick(p.id)}
              className="hidden"
            />
            <div className="text-sm font-medium">{p.name}</div>
            <div className="mono text-[11px] text-ink-500">{p.id}</div>
            <div className="mt-2 text-[11px] text-ink-500">
              凭据: <span className="mono">{p.credential_schema.map((f) => f.key).join(' · ')}</span>
            </div>
            <div className="text-[11px] text-ink-500">
              服务: {p.allowed_capabilities.join(' · ')}
            </div>
          </label>
        ))}
      </div>
    </div>
  );
}

// ─── Step 3 ─────────────────────────────────────────────────
function Step3({
  product,
  name,
  authPayload,
  onChangeName,
  onChangeField,
}: {
  product: VendorProduct;
  name: string;
  authPayload: Record<string, string>;
  onChangeName: (v: string) => void;
  onChangeField: (k: string, v: string) => void;
}) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5 space-y-4">
      <div className="text-sm text-ink-500">
        填写 <span className="mono text-brand-200">{product.id}</span> 板块的凭据字段（schema 由代码常量决定）。
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Field label="凭据名称 *" value={name} onChange={onChangeName} placeholder="方舟应用-生产" />
        {product.credential_schema.map((f) => (
          <Field
            key={f.key}
            label={`${f.label}${f.required ? ' *' : ''}${f.sensitive ? ' 🔒' : ''}`}
            value={authPayload[f.key] ?? ''}
            onChange={(v) => onChangeField(f.key, v)}
            sensitive={f.sensitive}
          />
        ))}
      </div>
    </div>
  );
}

// ─── Step 4 ─────────────────────────────────────────────────
function Step4({
  caps,
  bindings,
  setBindings,
}: {
  caps: Capability[];
  bindings: BindingDraft[];
  setBindings: React.Dispatch<React.SetStateAction<BindingDraft[]>>;
}) {
  function update(idx: number, patch: Partial<BindingDraft>) {
    setBindings((bs) => bs.map((b, i) => (i === idx ? { ...b, ...patch } : b)));
  }

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="text-sm text-ink-500 mb-3">
        勾选要绑定的服务并设置 tier / QPS / 配额。
      </div>
      <div className="overflow-hidden rounded-xl border border-ink-200">
        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-3 py-2 w-10"></th>
              <th className="px-3 py-2 text-left">能力</th>
              <th className="px-3 py-2 text-left">tier</th>
              <th className="px-3 py-2 text-right">QPS</th>
              <th className="px-3 py-2 text-right">日限额 (cents)</th>
              <th className="px-3 py-2 text-right">cost_basis</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {bindings.map((b, idx) => {
              const cap = caps.find((c) => c.id === b.capability);
              return (
                <tr key={b.capability}>
                  <td className="px-3 py-2">
                    <input
                      type="checkbox"
                      checked={b.selected}
                      onChange={(e) => update(idx, { selected: e.target.checked })}
                    />
                  </td>
                  <td className="px-3 py-2">
                    <span className="mono">{b.capability}</span>{' '}
                    {cap ? <span className="text-[11px] text-ink-500">· {cap.display_name}</span> : null}
                  </td>
                  <td className="px-3 py-2">
                    <select
                      disabled={!b.selected}
                      value={b.tier}
                      onChange={(e) => update(idx, { tier: e.target.value })}
                      className="rounded border border-ink-200 bg-white px-2 py-1 text-xs disabled:opacity-50"
                    >
                      <option>free</option>
                      <option>pro</option>
                      <option>enterprise</option>
                    </select>
                  </td>
                  <td className="px-3 py-2">
                    <input
                      type="number"
                      disabled={!b.selected}
                      value={b.qps_limit}
                      onChange={(e) => update(idx, { qps_limit: Number(e.target.value) })}
                      className="w-20 rounded border border-ink-200 bg-white px-2 py-1 text-xs text-right disabled:opacity-50"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <input
                      type="number"
                      disabled={!b.selected}
                      value={b.daily_limit_cents}
                      onChange={(e) => update(idx, { daily_limit_cents: Number(e.target.value) })}
                      className="w-28 rounded border border-ink-200 bg-white px-2 py-1 text-xs text-right disabled:opacity-50"
                    />
                  </td>
                  <td className="px-3 py-2">
                    <input
                      type="number"
                      step="0.01"
                      disabled={!b.selected}
                      value={b.cost_basis_cents}
                      onChange={(e) => update(idx, { cost_basis_cents: Number(e.target.value) })}
                      className="w-20 rounded border border-ink-200 bg-white px-2 py-1 text-xs text-right disabled:opacity-50"
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ─── Step 5 ─────────────────────────────────────────────────
function Step5({
  account,
  product,
  name,
  bindings,
}: {
  account: VendorAccount;
  product: VendorProduct;
  name: string;
  bindings: BindingDraft[];
}) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5 space-y-4">
      <div className="text-sm font-medium">提交摘要</div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3 text-sm">
        <Info label="主账号" value={`🪪 ${account.name} (acct_${account.id})`} />
        <Info label="业务板块" value={`${product.id} · ${product.name}`} />
        <Info label="将创建" value={`1 凭据 + ${bindings.length} 服务绑定（同事务）`} />
      </div>
      <div className="rounded-xl border border-ink-200 overflow-hidden">
        <div className="px-3 py-2 bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
          凭据
        </div>
        <table className="w-full text-sm">
          <tbody className="divide-y divide-ink-200 bg-white">
            <tr>
              <td className="px-3 py-2 w-32 text-xs text-ink-500">名称</td>
              <td className="px-3 py-2">{name}</td>
            </tr>
          </tbody>
        </table>
        <div className="px-3 py-2 bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500 border-t border-ink-200">
          服务绑定（{bindings.length}）
        </div>
        <table className="w-full text-sm">
          <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-3 py-2 text-left">能力</th>
              <th className="px-3 py-2 text-left">tier</th>
              <th className="px-3 py-2 text-right">QPS</th>
              <th className="px-3 py-2 text-right">日限额</th>
              <th className="px-3 py-2 text-right">cost</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200 bg-white">
            {bindings.map((b) => (
              <tr key={b.capability}>
                <td className="px-3 py-2 mono">{b.capability}</td>
                <td className="px-3 py-2">{b.tier}</td>
                <td className="px-3 py-2 text-right">{b.qps_limit}</td>
                <td className="px-3 py-2 text-right">{b.daily_limit_cents}</td>
                <td className="px-3 py-2 text-right">{b.cost_basis_cents}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-ink-200 bg-white p-3">
      <div className="text-xs text-ink-500">{label}</div>
      <div className="mt-1 text-sm">{value}</div>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  sensitive,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  sensitive?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        type={sensitive ? 'password' : 'text'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500"
      />
    </label>
  );
}
