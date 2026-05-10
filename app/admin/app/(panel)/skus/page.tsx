'use client';

import { useEffect, useState } from 'react';
import {
  api,
  type PlatformCategory,
  type PlatformService,
  type UpdatePricingReq,
  type VendorProduct,
} from '@/lib/admin-api';
import { fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';
import CreateSKUForm from './_create-form';

// 平台 SKU 管理：catalog.platform_services。SKU 是用户在"模型市场"
// 看到的服务条目，由(vendor_product, capability, [model]) 三元组绑定
// 上游。
export default function SKUsPage() {
  const [rows, setRows] = useState<PlatformService[]>([]);
  const [cats, setCats] = useState<PlatformCategory[]>([]);
  const [products, setProducts] = useState<VendorProduct[]>([]);
  const [filter, setFilter] = useState({ category: '', product: '', status: '', q: '' });
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [pricingFor, setPricingFor] = useState<PlatformService | null>(null);

  async function load() {
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.category) params.set('category_id', filter.category);
      if (filter.product) params.set('vendor_product_id', filter.product);
      if (filter.status) params.set('status', filter.status);
      if (filter.q) params.set('q', filter.q);
      params.set('limit', '500');
      const res = await api.get<{ data: PlatformService[] }>(
        `/api/admin/platform-services?${params}`,
      );
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    Promise.all([
      api.get<{ data: PlatformCategory[] }>('/api/admin/catalog/categories'),
      api.get<{ data: VendorProduct[] }>('/api/admin/catalog/products'),
    ])
      .then(([c, p]) => {
        setCats(c.data ?? []);
        setProducts([...(p.data ?? [])].sort((a, b) => a.id.localeCompare(b.id)));
      })
      .catch(() => {});
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="平台 SKU"
        subtitle="模型市场对外卖的服务条目。每条 SKU 绑定一组（业务板块 + 能力 + 上游模型）。"
        actions={
          <button
            onClick={() => setShowForm((s) => !s)}
            className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200"
          >
            {showForm ? '收起' : '+ 新增 SKU'}
          </button>
        }
      />

      {showForm ? (
        <CreateSKUForm
          categories={cats}
          products={products}
          onClose={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false);
            load();
          }}
        />
      ) : null}

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4 text-sm">
        <Filter
          label="平台分类"
          value={filter.category}
          onChange={(v) => setFilter({ ...filter, category: v })}
          options={[
            { value: '', label: '全部' },
            ...cats.map((c) => ({ value: c.id, label: c.name })),
          ]}
        />
        <Filter
          label="业务板块"
          value={filter.product}
          onChange={(v) => setFilter({ ...filter, product: v })}
          options={[
            { value: '', label: '全部' },
            ...products.map((p) => ({ value: p.id, label: p.id })),
          ]}
        />
        <Filter
          label="状态"
          value={filter.status}
          onChange={(v) => setFilter({ ...filter, status: v })}
          options={[
            { value: '', label: '全部' },
            { value: 'active', label: 'active' },
            { value: 'hidden', label: 'hidden' },
            { value: 'deprecated', label: 'deprecated' },
          ]}
        />
        <label className="flex flex-1 min-w-[180px] flex-col">
          <span className="text-xs text-ink-500">搜索</span>
          <input
            value={filter.q}
            onChange={(e) => setFilter({ ...filter, q: e.target.value })}
            placeholder="ID / 显示名"
            className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
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
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-700 bg-ink-800">
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">SKU</th>
              <th className="px-4 py-2.5 text-left">分类</th>
              <th className="px-4 py-2.5 text-left">板块</th>
              <th className="px-4 py-2.5 text-left">能力</th>
              <th className="px-4 py-2.5 text-left">上游模型</th>
              <th className="px-4 py-2.5 text-left">单位</th>
              <th className="px-4 py-2.5 text-right">retail</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-left">创建</th>
              <th className="px-4 py-2.5 text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={10} className="px-5 py-12 text-center text-ink-500">
                  无 SKU — 用上方 + 新增 SKU 创建
                </td>
              </tr>
            ) : (
              rows.map((s) => (
                <tr key={s.id}>
                  <td className="px-4 py-2.5">
                    <div className="text-ink-200">{s.display_name}</div>
                    <div className="mono text-[11px] text-ink-500">{s.id}</div>
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{s.category_id}</td>
                  <td className="px-4 py-2.5 mono text-xs">{s.vendor_product_id}</td>
                  <td className="px-4 py-2.5 mono text-xs">{s.capability}</td>
                  <td className="px-4 py-2.5 mono text-xs text-ink-400">
                    {s.upstream_model ?? '—'}
                  </td>
                  <td className="px-4 py-2.5 text-xs">{s.billing_unit}</td>
                  <td className="px-4 py-2.5 text-right text-xs">
                    {fmtRetail(s)}
                  </td>
                  <td className="px-4 py-2.5">
                    <StatusPill status={s.status} />
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">
                    {fmtDateTime(s.created_at)}
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => setPricingFor(s)}
                      className="rounded-md border border-ink-600 px-2 py-0.5 text-[11px] text-ink-200 hover:bg-ink-700"
                    >
                      改价
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {pricingFor ? (
        <PricingModal
          sku={pricingFor}
          onClose={() => setPricingFor(null)}
          onSaved={() => {
            setPricingFor(null);
            load();
          }}
        />
      ) : null}
    </div>
  );
}

function fmtRetail(s: PlatformService): string {
  // llm: in/out per 1k_tokens；其它: 主单价
  if (s.current_input_cents != null && s.current_output_cents != null) {
    return `${s.current_input_cents.toFixed(2)} / ${s.current_output_cents.toFixed(2)}`;
  }
  if (s.current_image_cents != null) {
    return `${s.current_image_cents.toFixed(2)}`;
  }
  if (s.current_input_cents != null) {
    return `${s.current_input_cents.toFixed(2)}`;
  }
  return '—';
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
        className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
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
      ? 'bg-emerald-500/20 text-emerald-300'
      : status === 'hidden'
        ? 'bg-ink-700 text-ink-200'
        : 'bg-rose-500/20 text-rose-300';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}

// 改价 modal — 追加一行新 effective_from=NOW 的 pricing；旧行 effective_until
// 在后端事务里自动收尾，所以前端不用关心。
//
// 输入字段是「新价」，旁边显示当前生效价做对比，让 admin 看清改了多少。
function PricingModal({
  sku,
  onClose,
  onSaved,
}: {
  sku: PlatformService;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [input, setInput] = useState<string>(sku.current_input_cents?.toString() ?? '');
  const [output, setOutput] = useState<string>(sku.current_output_cents?.toString() ?? '');
  const [image, setImage] = useState<string>(sku.current_image_cents?.toString() ?? '');
  const [notes, setNotes] = useState<string>('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isImageUnit = sku.billing_unit === 'image';
  const isLLMUnit = sku.billing_unit === '1k_tokens';

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const body: UpdatePricingReq = { notes: notes || undefined };
      if (input !== '') body.input_per_unit_cents = Number(input);
      if (output !== '') body.output_per_unit_cents = Number(output);
      if (image !== '') body.image_per_unit_cents = Number(image);
      if (
        body.input_per_unit_cents === undefined &&
        body.output_per_unit_cents === undefined &&
        body.image_per_unit_cents === undefined
      ) {
        setError('请至少填一个价格字段');
        setSubmitting(false);
        return;
      }
      await api.post(`/api/admin/platform-services/${sku.id}/pricing`, body);
      onSaved();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-4"
      role="dialog"
      aria-modal="true"
      onClick={onClose}
    >
      <form
        onSubmit={onSubmit}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-xl rounded-2xl border border-ink-700 bg-ink-800 p-6"
      >
        <div className="mb-1 text-sm font-semibold text-white">改价 · {sku.display_name}</div>
        <div className="mono mb-4 text-[11px] text-ink-500">
          {sku.id} · {sku.billing_unit}
        </div>

        <div className="space-y-3 text-sm">
          {(isLLMUnit || sku.billing_unit !== 'image') && (
            <PriceField
              label={isLLMUnit ? 'input_per_unit_cents（每 1k 输入）' : 'input_per_unit_cents'}
              current={sku.current_input_cents}
              value={input}
              onChange={setInput}
            />
          )}
          {isLLMUnit ? (
            <PriceField
              label="output_per_unit_cents（每 1k 输出）"
              current={sku.current_output_cents}
              value={output}
              onChange={setOutput}
            />
          ) : null}
          {isImageUnit ? (
            <PriceField
              label="image_per_unit_cents（每张）"
              current={sku.current_image_cents}
              value={image}
              onChange={setImage}
            />
          ) : null}

          <label className="block">
            <span className="text-xs text-ink-500">调价说明（写进 catalog.platform_pricing.notes）</span>
            <input
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="e.g. 厂商降价 / 季度调整"
              className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
            />
          </label>
        </div>

        {error ? <div className="mt-3 text-sm text-rose-400">{error}</div> : null}

        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-ink-700 px-4 py-2 text-sm text-ink-200 hover:bg-ink-700"
          >
            取消
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
          >
            {submitting ? '保存中…' : '应用新价'}
          </button>
        </div>
      </form>
    </div>
  );
}

function PriceField({
  label,
  current,
  value,
  onChange,
}: {
  label: string;
  current: number | undefined;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">
        {label}
        {current !== undefined ? (
          <span className="mono ml-2 text-[11px] text-ink-400">当前 {current.toFixed(4)}</span>
        ) : (
          <span className="ml-2 text-[11px] text-ink-400">当前 —</span>
        )}
      </span>
      <input
        type="number"
        step="0.0001"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="留空 = 不调整"
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}
