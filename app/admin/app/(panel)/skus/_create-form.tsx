'use client';

import { useMemo, useState } from 'react';
import {
  api,
  type CreatePlatformServiceReq,
  type PlatformCategory,
  type VendorProduct,
} from '@/lib/admin-api';

// 新增 SKU 内联表单。约束：
//   - vendor_product_id 必须存在于 catalog.Products
//   - capability 必须 ∈ products[vendor_product_id].allowed_capabilities
//   - 只列允许的能力（动态联动）
export default function CreateSKUForm({
  categories,
  products,
  onClose,
  onCreated,
}: {
  categories: PlatformCategory[];
  products: VendorProduct[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [productID, setProductID] = useState(products[0]?.id ?? '');
  const [form, setForm] = useState<Partial<CreatePlatformServiceReq>>({
    id: '',
    display_name: '',
    category_id: 'llm',
    capability: '',
    upstream_model: '',
    billing_unit: '1k_tokens',
    is_public: true,
    sort_order: 100,
  });
  const [pricing, setPricing] = useState({
    input: '',
    output: '',
    image: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const product = useMemo(
    () => products.find((p) => p.id === productID),
    [products, productID],
  );
  const allowed = product?.allowed_capabilities ?? [];

  // capability 在板块切换后自动重置为第一个允许项
  const capability = form.capability && allowed.includes(form.capability)
    ? form.capability
    : allowed[0] ?? '';

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const body: CreatePlatformServiceReq = {
        id: form.id ?? '',
        display_name: form.display_name ?? '',
        category_id: form.category_id ?? 'llm',
        description: form.description,
        vendor_product_id: productID,
        capability,
        upstream_model: form.upstream_model || undefined,
        billing_unit: form.billing_unit ?? '1k_tokens',
        context_window: form.context_window,
        max_output_tokens: form.max_output_tokens,
        is_public: form.is_public ?? true,
        sort_order: form.sort_order ?? 100,
        tags: form.tags,
        input_per_unit_cents: pricing.input ? Number(pricing.input) : undefined,
        output_per_unit_cents: pricing.output ? Number(pricing.output) : undefined,
        image_per_unit_cents: pricing.image ? Number(pricing.image) : undefined,
      };
      await api.post('/api/admin/platform-services', body);
      onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
      <div className="mb-3 flex items-center justify-between">
        <div className="text-sm font-semibold text-white">新增平台 SKU</div>
        <button type="button" onClick={onClose} className="text-xs text-ink-500 hover:text-ink-200">
          关闭
        </button>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Field
          label="SKU ID *"
          value={form.id ?? ''}
          onChange={(v) => setForm({ ...form, id: v })}
          placeholder="doubao-pro / claude-haiku-4 / ..."
          mono
        />
        <Field
          label="显示名 *"
          value={form.display_name ?? ''}
          onChange={(v) => setForm({ ...form, display_name: v })}
          placeholder="豆包 Pro"
        />
        <Select
          label="平台分类 *"
          value={form.category_id ?? 'llm'}
          onChange={(v) => setForm({ ...form, category_id: v })}
          options={categories.map((c) => ({ value: c.id, label: `${c.id} · ${c.name}` }))}
        />

        <Select
          label="业务板块 *"
          value={productID}
          onChange={setProductID}
          options={products.map((p) => ({ value: p.id, label: `${p.id} · ${p.name}` }))}
        />
        <Select
          label="能力 *"
          value={capability}
          onChange={(v) => setForm({ ...form, capability: v })}
          options={allowed.map((c) => ({ value: c, label: c }))}
          disabled={allowed.length === 0}
        />
        <Field
          label="上游模型 (llm 类填)"
          value={form.upstream_model ?? ''}
          onChange={(v) => setForm({ ...form, upstream_model: v })}
          placeholder="doubao-pro / claude-haiku-4-5"
          mono
        />

        <Select
          label="计费单位 *"
          value={form.billing_unit ?? '1k_tokens'}
          onChange={(v) => setForm({ ...form, billing_unit: v })}
          options={[
            { value: '1k_tokens', label: '1k_tokens' },
            { value: 'minute', label: 'minute' },
            { value: '1k_chars', label: '1k_chars' },
            { value: 'image', label: 'image' },
            { value: 'page', label: 'page' },
            { value: 'query', label: 'query' },
          ]}
        />
        <NumberField
          label="context_window"
          value={form.context_window}
          onChange={(v) => setForm({ ...form, context_window: v })}
        />
        <NumberField
          label="max_output_tokens"
          value={form.max_output_tokens}
          onChange={(v) => setForm({ ...form, max_output_tokens: v })}
        />
      </div>

      <div className="mt-4 mb-2 text-xs text-ink-500">初始零售价（cents per unit）</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Field
          label="input_per_unit"
          value={pricing.input}
          onChange={(v) => setPricing({ ...pricing, input: v })}
          placeholder="0.30"
        />
        <Field
          label="output_per_unit (llm)"
          value={pricing.output}
          onChange={(v) => setPricing({ ...pricing, output: v })}
          placeholder="1.20"
        />
        <Field
          label="image_per_unit (vision/image_gen)"
          value={pricing.image}
          onChange={(v) => setPricing({ ...pricing, image: v })}
          placeholder="0.05"
        />
      </div>

      {error ? <div className="mt-3 text-sm text-rose-400">{error}</div> : null}

      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-lg border border-ink-700 px-4 py-2 text-sm text-ink-200 hover:bg-ink-700"
        >
          取消
        </button>
        <button
          type="submit"
          disabled={submitting || !form.id || !form.display_name || !capability}
          className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
        >
          {submitting ? '创建中…' : '创建 SKU'}
        </button>
      </div>
    </form>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  mono,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  mono?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500 ${
          mono ? 'mono' : ''
        }`}
      />
    </label>
  );
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: number | undefined;
  onChange: (v: number | undefined) => void;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        type="number"
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value ? Number(e.target.value) : undefined)}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}

function Select({
  label,
  value,
  onChange,
  options,
  disabled,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  disabled?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500 disabled:opacity-50"
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
