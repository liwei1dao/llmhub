'use client';

import { useEffect, useMemo, useState } from 'react';
import { api, type Vendor } from '@/lib/admin-api';

// 当前阶段允许创建的厂商。
// 其余 vendor 在面板里以 "敬请期待" 灰态展示。
const ENABLED_VENDORS = new Set(['volc']);

// 厂商专属的填写指引：放置位置、推荐主体写法、文档/控制台跳转。
// key 为 vendor.id。
type VendorGuide = {
  badge: string; // 卡片右上角小角标，如 "豆包 / 方舟"
  helpTitle: string;
  helpBullets: string[];
  consoleLabel: string;
  consoleURL: string;
};

const VENDOR_GUIDE: Record<string, VendorGuide> = {
  volc: {
    badge: '豆包 / 方舟',
    helpTitle: '记录火山引擎 主账号 登录信息',
    helpBullets: [
      '主账号 = 你登录火山引擎控制台用的账号（手机号 / 邮箱）+ 密码',
      '当前仅做「运营手工巡检 / 后台对账时跳转用」，不会拿它去调任何业务 API',
      '余额 / 账单 的自动同步暂未启用，账号详情页里的余额数字需要管理员手工录入；后续接入火山 billing API 后再补 AK/SK 字段',
      '业务调用使用的 API key 走 凭据管理 里单独维护，不要混填到这里',
      '密码会经 Vault 加密落库；本机浏览器不留明文',
      '主体 / 公司 推荐填工商登记的公司全称，便于对账',
    ],
    consoleLabel: '前往火山引擎官网',
    consoleURL: 'https://www.volcengine.com/',
  },
};

// 新增主账号表单。
// 当前仅开放 volc（豆包/火山方舟），其它 vendor 灰态展示。
export default function CreateMasterForm({
  vendors,
  onClose,
  onCreated,
}: {
  vendors: Vendor[];
  onClose: () => void;
  onCreated: () => void;
}) {
  // 默认选中第一个允许的 vendor
  const firstEnabled = useMemo(
    () => vendors.find((v) => ENABLED_VENDORS.has(v.id))?.id ?? '',
    [vendors],
  );
  const [vendorID, setVendorID] = useState(firstEnabled);
  useEffect(() => {
    if (!vendorID && firstEnabled) setVendorID(firstEnabled);
  }, [firstEnabled, vendorID]);

  const [name, setName] = useState('');
  const [entity, setEntity] = useState('');
  const [authPayload, setAuthPayload] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const vendor = useMemo(() => vendors.find((v) => v.id === vendorID), [vendors, vendorID]);
  const schema = vendor?.master_auth_schema ?? [];
  const guide = vendor ? VENDOR_GUIDE[vendor.id] : undefined;

  const isEnabled = vendor && ENABLED_VENDORS.has(vendor.id);
  const requiredFilled =
    !!name.trim() &&
    !!isEnabled &&
    schema.every((f) => !f.required || (authPayload[f.key] ?? '').trim() !== '');

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!isEnabled || !vendor) return;
    setSubmitting(true);
    setError(null);
    try {
      await api.post('/api/admin/vendor-accounts', {
        vendor_id: vendor.id,
        name: name.trim(),
        entity: entity.trim(),
        console_url: vendor.console_url ?? '',
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
    <form onSubmit={onSubmit} className="rounded-2xl border border-ink-200 bg-white p-6">
      <div className="mb-4 flex items-center justify-between">
        <div>
          <div className="text-base font-semibold text-ink-900">新增主账号</div>
          <div className="mt-0.5 text-xs text-ink-500">
            「主账号」= 你在某厂商的根账号。承担余额查询/对账，不直接调业务。业务调用走 凭据管理。
          </div>
        </div>
        <button type="button" onClick={onClose} className="text-xs text-ink-500 hover:text-ink-800">
          关闭
        </button>
      </div>

      {/* ① 选厂商 */}
      <Step label="① 选厂商">
        {vendors.length === 0 ? (
          <div className="rounded-lg border border-ink-200 bg-ink-50 p-4 text-sm text-ink-500">
            正在加载厂商目录…
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-4">
            {vendors.map((v) => {
              const enabled = ENABLED_VENDORS.has(v.id);
              const selected = v.id === vendorID;
              const g = VENDOR_GUIDE[v.id];
              return (
                <label
                  key={v.id}
                  className={`relative cursor-pointer rounded-lg border p-3 text-left transition ${
                    !enabled
                      ? 'cursor-not-allowed border-ink-200 bg-ink-50 opacity-60'
                      : selected
                        ? 'border-brand-500 ring-2 ring-brand-500/30 bg-white'
                        : 'border-ink-200 bg-white hover:border-ink-400'
                  }`}
                >
                  <input
                    type="radio"
                    name="vendor"
                    value={v.id}
                    disabled={!enabled}
                    checked={selected}
                    onChange={() => {
                      setVendorID(v.id);
                      setAuthPayload({});
                    }}
                    className="sr-only"
                  />
                  <div className="flex items-center justify-between">
                    <div className="font-medium text-ink-900">{v.name}</div>
                    {enabled && g?.badge ? (
                      <span className="rounded-full bg-brand-50 px-1.5 py-0.5 text-[10px] text-brand-700">
                        {g.badge}
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-0.5 text-[11px] text-ink-500 mono">{v.id}</div>
                  {!enabled ? (
                    <div className="mt-1 text-[11px] text-ink-500">敬请期待</div>
                  ) : null}
                </label>
              );
            })}
          </div>
        )}
      </Step>

      {/* 帮助卡片 */}
      {guide ? (
        <div className="mt-3 rounded-lg border border-brand-200 bg-brand-50 p-4 text-xs text-ink-700">
          <div className="font-medium text-brand-700">💡 {guide.helpTitle}</div>
          <ul className="mt-2 list-disc space-y-1 pl-5 text-ink-700">
            {guide.helpBullets.map((b, i) => (
              <li key={i}>{b}</li>
            ))}
          </ul>
          <a
            href={guide.consoleURL}
            target="_blank"
            rel="noreferrer"
            className="mt-2 inline-flex items-center gap-1 text-brand-700 hover:underline"
          >
            ↗ {guide.consoleLabel}
          </a>
        </div>
      ) : null}

      {/* ② 主账号信息 */}
      <Step label="② 主账号信息" className="mt-5">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <Field
            label="名称（备注）*"
            value={name}
            onChange={setName}
            placeholder="例：火山·主账号 / 公司A 火山主"
            help="仅用于后台识别，可改"
          />
          <Field
            label="主体 / 公司"
            value={entity}
            onChange={setEntity}
            placeholder="工商登记公司全称"
            help="对账时以此匹配发票"
          />
        </div>
      </Step>

      {/* ③ AK/SK */}
      <Step
        label="③ 主账号 AK / SK"
        className="mt-5"
        sub={
          schema.length > 0 ? (
            <span className="mono text-[11px]">
              {schema.map((f) => f.key).join(' · ')}
            </span>
          ) : null
        }
      >
        {!isEnabled ? (
          <div className="rounded-lg border border-ink-200 bg-ink-50 p-4 text-sm text-ink-500">
            该厂商暂未开放，请先选 <span className="font-medium text-ink-800">火山引擎（豆包/方舟）</span>。
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {schema.map((f) => (
              <Field
                key={f.key}
                label={`${f.label}${f.required ? ' *' : ''}${f.sensitive ? ' 🔒' : ''}`}
                value={authPayload[f.key] ?? ''}
                onChange={(v) => setAuthPayload({ ...authPayload, [f.key]: v })}
                placeholder={f.sensitive ? '••••••••••' : ''}
                sensitive={f.sensitive}
                help={f.sensitive ? '仅在创建时上传，落库前由 Vault 加密' : undefined}
              />
            ))}
          </div>
        )}
      </Step>

      {error ? (
        <div className="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="mt-6 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-lg border border-ink-200 px-4 py-2 text-sm text-ink-800 hover:bg-ink-100"
        >
          取消
        </button>
        <button
          type="submit"
          disabled={submitting || !requiredFilled}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 disabled:opacity-50"
        >
          {submitting ? '创建中…' : '创建主账号'}
        </button>
      </div>
    </form>
  );
}

function Step({
  label,
  sub,
  className,
  children,
}: {
  label: string;
  sub?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div className={className}>
      <div className="mb-2 flex items-baseline justify-between">
        <div className="text-xs font-medium uppercase tracking-wider text-ink-500">{label}</div>
        {sub ? <div className="text-ink-500">{sub}</div> : null}
      </div>
      {children}
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
  sensitive,
  help,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  sensitive?: boolean;
  help?: string;
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        type={sensitive ? 'password' : 'text'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete={sensitive ? 'new-password' : 'off'}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
      />
      {help ? <span className="mt-1 block text-[11px] text-ink-500">{help}</span> : null}
    </label>
  );
}
