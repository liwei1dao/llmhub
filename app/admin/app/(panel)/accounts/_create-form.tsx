'use client';

import { useMemo, useState } from 'react';
import { api, type Vendor } from '@/lib/admin-api';

// 新增主账号表单：根据所选 vendor 的 master_auth_schema 动态出字段。
export default function CreateMasterForm({
  vendors,
  onClose,
  onCreated,
}: {
  vendors: Vendor[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [vendorID, setVendorID] = useState(vendors[0]?.id ?? '');
  const [name, setName] = useState('');
  const [entity, setEntity] = useState('');
  const [authPayload, setAuthPayload] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const vendor = useMemo(() => vendors.find((v) => v.id === vendorID), [vendors, vendorID]);
  const schema = vendor?.master_auth_schema ?? [];

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.post('/api/admin/vendor-accounts', {
        vendor_id: vendorID,
        name,
        entity,
        console_url: vendor?.console_url ?? '',
        auth_payload: authPayload,
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
        <div className="text-sm font-semibold text-white">新增主账号</div>
        <button
          type="button"
          onClick={onClose}
          className="text-xs text-ink-500 hover:text-ink-200"
        >
          关闭
        </button>
      </div>

      <div className="mb-4">
        <div className="text-xs text-ink-500 mb-2">① 选厂商</div>
        <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-6">
          {vendors.map((v) => (
            <label
              key={v.id}
              className={`cursor-pointer rounded-lg border p-3 text-center text-sm ${
                v.id === vendorID
                  ? 'border-brand-500 ring-2 ring-brand-500/30 bg-ink-900'
                  : 'border-ink-700 bg-ink-900 hover:border-ink-500'
              }`}
            >
              <input
                type="radio"
                name="vendor"
                value={v.id}
                checked={v.id === vendorID}
                onChange={() => {
                  setVendorID(v.id);
                  setAuthPayload({});
                }}
                className="hidden"
              />
              <div className="font-medium">{v.name}</div>
              <div className="mt-0.5 text-[11px] text-ink-500 mono">{v.id}</div>
            </label>
          ))}
        </div>
      </div>

      <div className="mb-2 text-xs text-ink-500">② 主账号信息</div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Field label="名称（备注）*" value={name} onChange={setName} placeholder="公司A 火山主账号" />
        <Field label="主体 / 公司" value={entity} onChange={setEntity} placeholder="上海某某科技" />
      </div>

      <div className="mb-2 mt-4 text-xs text-ink-500">
        ③ 主账号鉴权（vendor schema：<span className="mono">{schema.map((f) => f.key).join(', ')}</span>）
      </div>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {schema.map((f) => (
          <Field
            key={f.key}
            label={`${f.label}${f.required ? ' *' : ''}${f.sensitive ? ' 🔒' : ''}`}
            value={authPayload[f.key] ?? ''}
            onChange={(v) => setAuthPayload({ ...authPayload, [f.key]: v })}
            sensitive={f.sensitive}
          />
        ))}
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
          disabled={submitting || !name || !vendorID}
          className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
        >
          {submitting ? '创建中…' : '创建主账号'}
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
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}
