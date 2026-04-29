'use client';

import { useState } from 'react';
import { api } from '@/lib/admin-api';

export default function CreateAccountForm({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState({
    provider_id: 'deepseek',
    tier: 'pro',
    origin: 'manual',
    supported_capabilities: 'chat,embedding',
    quota_total_cents: 1000000,
    daily_limit_cents: 50000,
    qps_limit: 20,
    cost_basis_cents: 100,
    api_key_ref: 'devkey://sk-upstream-changeme',
  });
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.post('/api/admin/pool/accounts', {
        ...form,
        supported_capabilities: form.supported_capabilities
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      });
      onCreated();
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
        <div className="text-sm font-semibold text-white">新增账号</div>
        <button
          type="button"
          onClick={onClose}
          className="text-xs text-ink-500 hover:text-ink-200"
        >
          关闭
        </button>
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Field label="provider_id" value={form.provider_id} onChange={(v) => setForm({ ...form, provider_id: v })} />
        <Field label="tier" value={form.tier} onChange={(v) => setForm({ ...form, tier: v })} />
        <Field label="origin" value={form.origin} onChange={(v) => setForm({ ...form, origin: v })} />
        <Field
          label="supported_capabilities (逗号分隔)"
          value={form.supported_capabilities}
          onChange={(v) => setForm({ ...form, supported_capabilities: v })}
        />
        <NumberField
          label="quota_total_cents"
          value={form.quota_total_cents}
          onChange={(v) => setForm({ ...form, quota_total_cents: v })}
        />
        <NumberField
          label="daily_limit_cents"
          value={form.daily_limit_cents}
          onChange={(v) => setForm({ ...form, daily_limit_cents: v })}
        />
        <NumberField
          label="qps_limit"
          value={form.qps_limit}
          onChange={(v) => setForm({ ...form, qps_limit: v })}
        />
        <NumberField
          label="cost_basis_cents (元/万 token)"
          value={form.cost_basis_cents}
          onChange={(v) => setForm({ ...form, cost_basis_cents: v })}
        />
        <Field
          label="api_key_ref (Vault 引用)"
          value={form.api_key_ref}
          onChange={(v) => setForm({ ...form, api_key_ref: v })}
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
          disabled={submitting}
          className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
        >
          {submitting ? '创建中…' : '创建'}
        </button>
      </div>
    </form>
  );
}

function Field({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
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
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}
