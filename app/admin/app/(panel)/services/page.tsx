'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  api,
  type CreatePlatformServiceReq,
  type PlatformService,
  type ServiceModule,
  type ServiceModuleModel,
  type ServiceModuleRegion,
} from '@/lib/admin-api';

// 服务列表（平台后台）
//
// 这页面让运营做两件事：
//   1. 看代码侧注册了哪些「服务模块」（vendor_product × capability），
//      每个模块声明的可用模型 / 节点，以及当前已上架 SKU 数。
//   2. 在一个模块的参数空间内挑选 (模型, 节点) → 创建对应的 SKU。
//
// 用户侧的 /services 页拉的是 SKU；这里维护的是 SKU 的"上架/下架"和定价。
export default function AdminServicesPage() {
  const [modules, setModules] = useState<ServiceModule[] | null>(null);
  const [skus, setSkus] = useState<PlatformService[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [pickModule, setPickModule] = useState<ServiceModule | null>(null);

  async function load() {
    try {
      const [m, s] = await Promise.all([
        api.get<{ data: ServiceModule[] }>('/api/admin/service-modules'),
        api.get<{ data: PlatformService[] }>('/api/admin/platform-services?limit=500'),
      ]);
      setModules(m.data ?? []);
      setSkus(s.data ?? []);
    } catch (e) {
      setError((e as Error).message);
    }
  }

  useEffect(() => {
    load();
  }, []);

  // 按模块 id 把已上架 SKU 索引起来，渲染明细 + "添加服务"表单去重时用。
  const skusByModule = useMemo(() => {
    const m: Record<string, PlatformService[]> = {};
    if (!skus) return m;
    for (const s of skus) {
      const key = `${s.vendor_product_id}.${s.capability}`;
      (m[key] ??= []).push(s);
    }
    return m;
  }, [skus]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">服务列表</h1>
        <p className="mt-1 text-sm text-ink-500">
          代码侧注册的服务模块一览。每个模块声明它能开放给运营的「可用模型」+「可用节点」，运营只能在这个参数空间里挑选 → 落 SKU。
        </p>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-300 bg-rose-50 px-4 py-3 text-sm text-rose-800">
          {error}
        </div>
      ) : null}

      {modules === null ? (
        <div className="text-sm text-ink-500">加载中…</div>
      ) : modules.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-ink-300 bg-white px-5 py-12 text-center text-sm text-ink-500">
          代码里还没注册任何服务模块。
        </div>
      ) : (
        <div className="space-y-4">
          {modules.map((m) => (
            <ModuleCard
              key={m.id}
              mod={m}
              skus={skusByModule[m.id] ?? []}
              onAddService={() => setPickModule(m)}
              onChanged={load}
            />
          ))}
        </div>
      )}

      {pickModule ? (
        <AddServiceModal
          mod={pickModule}
          existingSkus={skusByModule[pickModule.id] ?? []}
          onClose={() => setPickModule(null)}
          onCreated={async () => {
            setPickModule(null);
            await load();
          }}
        />
      ) : null}
    </div>
  );
}

function ModuleCard({
  mod,
  skus,
  onAddService,
  onChanged,
}: {
  mod: ServiceModule;
  skus: PlatformService[];
  onAddService: () => void;
  onChanged: () => Promise<void> | void;
}) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold">{mod.display_name}</h2>
            <span className="mono text-[11px] text-ink-500">{mod.id}</span>
            <ImplementedBadge implemented={mod.implemented} />
          </div>
          <div className="mt-1 text-xs text-ink-500">
            {mod.vendor_name ?? mod.vendor_id} · {mod.capability_name ?? mod.capability} ·{' '}
            计费单位 {mod.default_billing_unit} · 已上架 {mod.listed_skus} 条
          </div>
          {mod.description ? (
            <p className="mt-2 text-xs text-ink-600">{mod.description}</p>
          ) : null}
        </div>
        <button
          onClick={onAddService}
          disabled={!mod.implemented}
          title={mod.implemented ? '' : '代码侧未注册适配器，禁止上架（上了用户会拿 no_binding_available）'}
          className="shrink-0 rounded-lg bg-ink-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-ink-800 disabled:cursor-not-allowed disabled:bg-ink-300"
        >
          添加服务
        </button>
      </div>

      <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-wider text-ink-500">
            可用模型（{mod.available_models?.length ?? 0}）
          </div>
          <ul className="mt-1 space-y-1 text-xs text-ink-700">
            {(mod.available_models ?? []).map((m) => (
              <li key={m.id} className="flex items-baseline justify-between gap-2">
                <span>
                  {m.display_name}{' '}
                  <code className="mono text-[10px] text-ink-500">{m.id}</code>
                </span>
                <span className="mono text-[10px] text-ink-500">
                  {m.context_window ? `${(m.context_window / 1024).toFixed(0)}k` : ''}
                  {m.default_input_cents !== undefined
                    ? ` · ¥${(m.default_input_cents / 100).toFixed(4)}/${(m.default_output_cents ?? 0) / 100}`
                    : ''}
                </span>
              </li>
            ))}
            {(mod.available_models ?? []).length === 0 ? (
              <li className="text-ink-400">—</li>
            ) : null}
          </ul>
        </div>
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-wider text-ink-500">
            可用节点（{mod.available_regions?.length ?? 0}）
          </div>
          <ul className="mt-1 space-y-1 text-xs text-ink-700">
            {(mod.available_regions ?? []).map((r) => (
              <li key={r.id} className="flex items-baseline justify-between gap-2">
                <span>
                  {r.display_name}{' '}
                  <code className="mono text-[10px] text-ink-500">{r.id}</code>
                  {r.default ? (
                    <span className="ml-1 rounded bg-emerald-100 px-1 py-0.5 text-[9px] text-emerald-700">
                      默认
                    </span>
                  ) : null}
                </span>
                <span className="mono text-[10px] text-ink-400">{r.endpoint}</span>
              </li>
            ))}
            {(mod.available_regions ?? []).length === 0 ? (
              <li className="text-ink-400">—</li>
            ) : null}
          </ul>
        </div>
      </div>

      {skus.length > 0 ? (
        <div className="mt-4 border-t border-ink-200 pt-3">
          <div className="text-[11px] font-semibold uppercase tracking-wider text-ink-500">
            已上架 SKU
          </div>
          <table className="mt-1 w-full text-xs">
            <thead className="text-[10px] uppercase tracking-wider text-ink-400">
              <tr className="border-b border-ink-200">
                <th className="py-1.5 text-left">ID / 名称</th>
                <th className="py-1.5 text-left">上游模型</th>
                <th className="py-1.5 text-left">状态 / 可见</th>
                <th className="py-1.5 text-right">输入 / 输出 单价</th>
                <th className="py-1.5 text-right">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-100">
              {skus.map((s) => (
                <SKURow key={s.id} sku={s} onChanged={onChanged} />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}

function ImplementedBadge({ implemented }: { implemented: boolean }) {
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium ${
        implemented
          ? 'bg-emerald-100 text-emerald-700'
          : 'bg-amber-100 text-amber-700'
      }`}
    >
      {implemented ? '代码已实现' : '未实现'}
    </span>
  );
}

function SKURow({ sku, onChanged }: { sku: PlatformService; onChanged: () => Promise<void> | void }) {
  const [busy, setBusy] = useState(false);
  // 状态翻转 + 是否对用户可见，是运营最频繁要做的两件事。
  async function toggleStatus(next: 'active' | 'hidden') {
    setBusy(true);
    try {
      await api.patch(`/api/admin/platform-services/${sku.id}`, { status: next });
      await onChanged();
    } finally {
      setBusy(false);
    }
  }
  async function togglePublic() {
    setBusy(true);
    try {
      await api.patch(`/api/admin/platform-services/${sku.id}`, { is_public: !sku.is_public });
      await onChanged();
    } finally {
      setBusy(false);
    }
  }
  return (
    <tr>
      <td className="py-1.5">
        <div className="text-ink-800">{sku.display_name}</div>
        <code className="mono text-[10px] text-ink-500">{sku.id}</code>
      </td>
      <td className="py-1.5 text-ink-600">{sku.upstream_model || '—'}</td>
      <td className="py-1.5">
        <div className="flex items-center gap-1">
          <span
            className={`rounded px-1.5 py-0.5 text-[10px] ${
              sku.status === 'active'
                ? 'bg-emerald-100 text-emerald-700'
                : sku.status === 'hidden'
                  ? 'bg-amber-100 text-amber-700'
                  : 'bg-ink-200 text-ink-700'
            }`}
          >
            {sku.status}
          </span>
          <span
            className={`rounded px-1.5 py-0.5 text-[10px] ${
              sku.is_public ? 'bg-brand-100 text-brand-700' : 'bg-ink-200 text-ink-500'
            }`}
          >
            {sku.is_public ? '可见' : '隐藏'}
          </span>
        </div>
      </td>
      <td className="py-1.5 text-right text-ink-700">
        {sku.current_input_cents !== undefined
          ? `${sku.current_input_cents.toFixed(2)}`
          : '—'}{' '}
        /{' '}
        {sku.current_output_cents !== undefined
          ? sku.current_output_cents.toFixed(2)
          : '—'}
        <span className="text-[10px] text-ink-400"> 分</span>
      </td>
      <td className="py-1.5 text-right">
        <div className="inline-flex gap-1">
          <button
            disabled={busy}
            onClick={() => toggleStatus(sku.status === 'active' ? 'hidden' : 'active')}
            className="rounded border border-ink-300 px-2 py-0.5 text-[10px] hover:bg-ink-100 disabled:opacity-50"
          >
            {sku.status === 'active' ? '下架' : '上架'}
          </button>
          <button
            disabled={busy}
            onClick={togglePublic}
            className="rounded border border-ink-300 px-2 py-0.5 text-[10px] hover:bg-ink-100 disabled:opacity-50"
          >
            {sku.is_public ? '隐藏' : '显示'}
          </button>
        </div>
      </td>
    </tr>
  );
}

function AddServiceModal({
  mod,
  existingSkus,
  onClose,
  onCreated,
}: {
  mod: ServiceModule;
  existingSkus: PlatformService[];
  onClose: () => void;
  onCreated: () => void | Promise<void>;
}) {
  const models = mod.available_models ?? [];
  const regions = mod.available_regions ?? [];
  const defaultRegionId = regions.find((r) => r.default)?.id ?? regions[0]?.id ?? '';

  const [modelId, setModelId] = useState<string>(models[0]?.id ?? '');
  const [regionId, setRegionId] = useState<string>(defaultRegionId);
  const [skuId, setSkuId] = useState<string>('');
  const [displayName, setDisplayName] = useState<string>('');
  const [inputCents, setInputCents] = useState<string>('');
  const [outputCents, setOutputCents] = useState<string>('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const model = useMemo(() => models.find((m) => m.id === modelId), [models, modelId]);
  const region = useMemo(() => regions.find((r) => r.id === regionId), [regions, regionId]);

  // 选模型后自动带入：SKU id 建议、展示名、单价默认值。
  // 命名约定：单节点时直接用模型 slug；多节点时拼上 region 后缀以保平台唯一。
  useEffect(() => {
    if (!model) return;
    const suggestedID =
      regions.length <= 1 ? model.id : `${model.id}-${regionId || defaultRegionId}`;
    setSkuId(suggestedID);
    setDisplayName(model.display_name);
    setInputCents(
      model.default_input_cents !== undefined ? String(model.default_input_cents) : ''
    );
    setOutputCents(
      model.default_output_cents !== undefined ? String(model.default_output_cents) : ''
    );
  }, [model, regionId, regions.length, defaultRegionId]);

  // 已经上架过相同 (模型, 节点) 的就拦住，免得运营误点产生重复 SKU。
  const dup = useMemo(() => {
    if (!model) return null;
    return existingSkus.find(
      (s) => s.upstream_model === model.upstream_model && s.id === skuId
    );
  }, [existingSkus, model, skuId]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!model) return;
    setError(null);
    setSubmitting(true);
    try {
      const body: CreatePlatformServiceReq = {
        id: skuId,
        category_id: mod.category_id ?? '',
        display_name: displayName,
        vendor_product_id: mod.vendor_product_id,
        capability: mod.capability,
        upstream_model: model.upstream_model,
        billing_unit: mod.default_billing_unit,
        context_window: model.context_window,
        max_output_tokens: model.max_output_tokens,
        is_public: true,
        sort_order: 100,
        tags: model.tags,
        input_per_unit_cents: inputCents ? Number(inputCents) : undefined,
        output_per_unit_cents: outputCents ? Number(outputCents) : undefined,
      };
      await api.post('/api/admin/platform-services', body);
      await onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink-900/40 px-4 py-8">
      <div className="flex max-h-[640px] w-full max-w-2xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl">
        <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3">
          <div>
            <div className="text-base font-semibold">添加服务 — {mod.display_name}</div>
            <div className="text-[11px] text-ink-500">
              在模块声明的可用模型 / 节点里选一组 → 落一行 catalog.platform_services。
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg border border-ink-300 px-3 py-1 text-xs hover:bg-ink-100"
          >
            关闭
          </button>
        </div>

        <form onSubmit={submit} className="space-y-4 overflow-y-auto p-5 text-sm">
          <Field label="模型版本">
            <select
              value={modelId}
              onChange={(e) => setModelId(e.target.value)}
              className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm"
            >
              {models.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.display_name} ({m.id})
                </option>
              ))}
            </select>
            {model ? (
              <div className="mt-1 text-[11px] text-ink-500">
                上游 model: <code className="mono">{model.upstream_model}</code>
                {model.context_window ? ` · 上下文 ${model.context_window.toLocaleString()}` : ''}
                {model.max_output_tokens ? ` · 最大输出 ${model.max_output_tokens.toLocaleString()}` : ''}
              </div>
            ) : null}
          </Field>

          <Field label="节点">
            <select
              value={regionId}
              onChange={(e) => setRegionId(e.target.value)}
              className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm"
            >
              {regions.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.display_name} ({r.id})
                </option>
              ))}
            </select>
            {region ? (
              <div className="mt-1 text-[11px] text-ink-500">
                endpoint: <code className="mono">{region.endpoint}</code>
              </div>
            ) : null}
          </Field>

          <div className="grid grid-cols-2 gap-3">
            <Field label="SKU ID (平台唯一)">
              <input
                value={skuId}
                onChange={(e) => setSkuId(e.target.value)}
                className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm mono"
              />
            </Field>
            <Field label="展示名">
              <input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm"
              />
            </Field>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Field label={`输入单价（分 / ${mod.default_billing_unit}）`}>
              <input
                type="number"
                step="0.0001"
                value={inputCents}
                onChange={(e) => setInputCents(e.target.value)}
                className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm mono"
              />
            </Field>
            <Field label={`输出单价（分 / ${mod.default_billing_unit}）`}>
              <input
                type="number"
                step="0.0001"
                value={outputCents}
                onChange={(e) => setOutputCents(e.target.value)}
                className="w-full rounded-lg border border-ink-300 px-3 py-2 text-sm mono"
              />
            </Field>
          </div>

          {dup ? (
            <div className="rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-800">
              已存在同 ID 的 SKU（{dup.display_name}），换一个 ID 或先下架旧的。
            </div>
          ) : null}

          {error ? (
            <div className="rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-800">
              {error}
            </div>
          ) : null}

          <div className="flex items-center justify-end gap-2 border-t border-ink-200 pt-3">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-ink-300 px-3 py-1.5 text-xs hover:bg-ink-100"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={submitting || !!dup || !model}
              className="rounded-lg bg-ink-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-ink-800 disabled:opacity-50"
            >
              {submitting ? '创建中…' : '创建并上架'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <div className="mb-1 text-xs font-medium text-ink-700">{label}</div>
      {children}
    </label>
  );
}
