'use client';

import { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import {
  api,
  type AdminSubscription,
  type AdminUserDetail,
  type AdminUserWallet,
  type GrantSubscriptionReq,
  type PlatformService,
} from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../../_components/page-header';

// 用户详情：核心字段 + 订阅列表 + 开订阅表单。
// 把订阅拿到这里展示而不是单独页面，是因为它们的主要查询场景都是
// 「我要看某用户买了什么 / 给他加点配额」，按用户聚合最直接。
export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [user, setUser] = useState<AdminUserDetail | null>(null);
  const [wallet, setWallet] = useState<AdminUserWallet | null>(null);
  const [subs, setSubs] = useState<AdminSubscription[]>([]);
  const [skus, setSkus] = useState<PlatformService[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showGrant, setShowGrant] = useState(false);

  async function load() {
    setError(null);
    try {
      const [u, w, s, k] = await Promise.all([
        api.get<AdminUserDetail>(`/api/admin/users/${id}`),
        api.get<AdminUserWallet>(`/api/admin/users/${id}/wallet`),
        api.get<{ data: AdminSubscription[] }>(`/api/admin/users/${id}/subscriptions`),
        api.get<{ data: PlatformService[] }>(`/api/admin/platform-services?limit=500`),
      ]);
      setUser(u);
      setWallet(w);
      setSubs(s.data ?? []);
      setSkus(k.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  async function onCancel(s: AdminSubscription) {
    if (!confirm(`取消订阅 ${s.sku_id}? 该用户的 SDK 立即拿不到新 lease。`)) return;
    try {
      await api.delete(`/api/admin/subscriptions/${s.id}`);
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function onAddQuota(s: AdminSubscription) {
    const raw = prompt(`为 ${s.sku_id} 新增多少配额？（当前 quota_total=${s.quota_total}）`);
    if (!raw) return;
    const add = Number(raw);
    if (!Number.isFinite(add) || add <= 0) {
      alert('请输入一个正数');
      return;
    }
    try {
      await api.patch(`/api/admin/subscriptions/${s.id}`, {
        quota_total: s.quota_total + add,
      });
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function onToggleStatus(s: AdminSubscription) {
    const target = s.status === 'active' ? 'suspended' : 'active';
    try {
      await api.patch(`/api/admin/subscriptions/${s.id}`, { status: target });
      await load();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={user ? `用户 #${user.id}` : '加载中…'}
        subtitle={user?.email ?? user?.phone ?? ''}
      />

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      {user ? <UserCard u={user} /> : null}

      {/* ── 钱包 / 消费 ─────────────────────────────────── */}
      {wallet ? <WalletCard w={wallet} /> : null}

      {/* ── 订阅区块 ─────────────────────────────────────── */}
      <div className="rounded-2xl border border-ink-700 bg-ink-800">
        <div className="flex items-center justify-between border-b border-ink-700 px-5 py-3">
          <div className="text-sm font-semibold text-white">订阅</div>
          <button
            onClick={() => setShowGrant((v) => !v)}
            className="rounded-lg bg-white px-4 py-1.5 text-xs font-medium text-ink-900 hover:bg-ink-200"
          >
            {showGrant ? '收起' : '+ 开订阅'}
          </button>
        </div>

        {showGrant && user ? (
          <GrantForm
            userId={user.id}
            skus={skus}
            onClose={() => setShowGrant(false)}
            onCreated={() => {
              setShowGrant(false);
              load();
            }}
          />
        ) : null}

        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">SKU</th>
              <th className="px-4 py-2.5 text-left">套餐</th>
              <th className="px-4 py-2.5 text-right">配额（已用 / 总）</th>
              <th className="px-4 py-2.5 text-right">QPS</th>
              <th className="px-4 py-2.5 text-left">周期截止</th>
              <th className="px-4 py-2.5 text-left">状态</th>
              <th className="px-4 py-2.5 text-right">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {subs.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-10 text-center text-ink-500">
                  尚无订阅 — 用上方 + 开订阅 按钮创建一条
                </td>
              </tr>
            ) : (
              subs.map((s) => (
                <tr key={s.id}>
                  <td className="px-4 py-2.5">
                    <div className="text-ink-200">{s.sku_id}</div>
                    {s.plan_name ? (
                      <div className="text-[11px] text-ink-500">{s.plan_name}</div>
                    ) : null}
                  </td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{s.plan_kind}</td>
                  <td className="px-4 py-2.5 text-right mono text-xs">
                    {s.quota_used.toLocaleString()} / {s.quota_total.toLocaleString()}
                  </td>
                  <td className="px-4 py-2.5 text-right mono text-xs">{s.qps_limit}</td>
                  <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(s.period_end)}</td>
                  <td className="px-4 py-2.5">
                    <SubStatusPill status={s.status} />
                  </td>
                  <td className="px-4 py-2.5 text-right">
                    <div className="inline-flex flex-wrap justify-end gap-1.5">
                      <button
                        onClick={() => onAddQuota(s)}
                        disabled={s.status !== 'active'}
                        className="rounded-md border border-ink-600 px-2 py-0.5 text-[11px] text-ink-200 hover:bg-ink-700 disabled:opacity-40"
                      >
                        加配额
                      </button>
                      <button
                        onClick={() => onToggleStatus(s)}
                        className="rounded-md border border-ink-600 px-2 py-0.5 text-[11px] text-ink-200 hover:bg-ink-700"
                      >
                        {s.status === 'active' ? '暂停' : '恢复'}
                      </button>
                      <button
                        onClick={() => onCancel(s)}
                        disabled={s.status === 'cancelled'}
                        className="rounded-md border border-rose-800 px-2 py-0.5 text-[11px] text-rose-300 hover:bg-rose-950 disabled:opacity-40"
                      >
                        取消
                      </button>
                    </div>
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

function UserCard({ u }: { u: AdminUserDetail }) {
  return (
    <div className="grid grid-cols-2 gap-4 rounded-2xl border border-ink-700 bg-ink-800 p-5 md:grid-cols-4">
      <Field label="状态" value={u.status} />
      <Field label="风险分" value={String(u.risk_score)} />
      <Field label="QPS 上限" value={String(u.qps_limit)} />
      <Field label="日消费上限" value={fmtCents(u.daily_spend_limit_cents)} />
      <Field label="注册时间" value={fmtDateTime(u.created_at)} />
      <Field label="最近登录" value={fmtDateTime(u.last_login_at)} />
      <Field label="实名等级" value={String(u.realname_level ?? 0)} />
      <Field label="昵称" value={u.display_name ?? '-'} />
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11px] text-ink-500">{label}</div>
      <div className="text-sm text-ink-200">{value}</div>
    </div>
  );
}

function WalletCard({ w }: { w: AdminUserWallet }) {
  if (!w.account_exists) {
    return (
      <div className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
        <div className="text-sm font-semibold text-white mb-2">💰 钱包</div>
        <div className="text-xs text-ink-500">用户尚未发起过任何充值，钱包账户未创建。</div>
      </div>
    );
  }
  const currency = w.currency || 'CNY';
  return (
    <div className="rounded-2xl border border-ink-700 bg-ink-800">
      <div className="border-b border-ink-700 px-5 py-3 text-sm font-semibold text-white">
        💰 钱包 · 消费（最近 30 日）
      </div>
      <div className="grid grid-cols-2 gap-4 px-5 py-4 md:grid-cols-5">
        <Stat label="可用余额" value={fmtCents(w.balance_cents ?? 0, currency)} tone="emerald" />
        <Stat label="冻结" value={fmtCents(w.frozen_cents ?? 0, currency)} />
        <Stat label="累计充值" value={fmtCents(w.total_recharged_cents ?? 0, currency)} />
        <Stat label="累计消费" value={fmtCents(w.total_spent_cents ?? 0, currency)} />
        <Stat
          label="近 30 日消费"
          value={fmtCents(w.spent_30d_cents ?? 0, currency)}
          sub={`${(w.calls_30d ?? 0).toLocaleString()} 次调用`}
        />
      </div>

      {/* 最近充值 */}
      <div className="border-t border-ink-700">
        <div className="px-5 py-2.5 text-xs uppercase tracking-wider text-ink-500">
          最近充值（{w.recharges.length}）
        </div>
        {w.recharges.length === 0 ? (
          <div className="px-5 pb-4 pt-1 text-sm text-ink-500">—</div>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
              <tr>
                <th className="px-4 py-2 text-left">订单号</th>
                <th className="px-4 py-2 text-left">渠道</th>
                <th className="px-4 py-2 text-right">金额</th>
                <th className="px-4 py-2 text-left">状态</th>
                <th className="px-4 py-2 text-left">支付时间</th>
                <th className="px-4 py-2 text-left">创建</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-700">
              {w.recharges.map((r) => (
                <tr key={r.order_no}>
                  <td className="px-4 py-2 mono text-xs text-ink-300">{r.order_no}</td>
                  <td className="px-4 py-2 text-xs">{r.channel}</td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {fmtCents(r.amount_cents, currency)}
                  </td>
                  <td className="px-4 py-2">
                    <RechargeStatusPill status={r.status} />
                  </td>
                  <td className="px-4 py-2 text-xs text-ink-500">
                    {r.paid_at ? fmtDateTime(r.paid_at) : '-'}
                  </td>
                  <td className="px-4 py-2 text-xs text-ink-500">{fmtDateTime(r.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: 'emerald';
}) {
  const valueCls = tone === 'emerald' ? 'text-emerald-300' : 'text-ink-100';
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wider text-ink-500">{label}</div>
      <div className={`mt-0.5 text-base font-medium tabular-nums ${valueCls}`}>{value}</div>
      {sub ? <div className="text-[11px] text-ink-500">{sub}</div> : null}
    </div>
  );
}

function RechargeStatusPill({ status }: { status: string }) {
  const tone =
    status === 'paid'
      ? 'bg-emerald-500/20 text-emerald-300'
      : status === 'pending'
        ? 'bg-amber-500/20 text-amber-300'
        : status === 'failed' || status === 'cancelled' || status === 'refunded'
          ? 'bg-rose-500/20 text-rose-300'
          : 'bg-ink-700 text-ink-300';
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] ${tone}`}>{status}</span>
  );
}

function SubStatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-500/20 text-emerald-300'
      : status === 'suspended'
        ? 'bg-amber-500/20 text-amber-300'
        : status === 'cancelled'
          ? 'bg-rose-500/20 text-rose-300'
          : 'bg-ink-700 text-ink-200';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}

function GrantForm({
  userId,
  skus,
  onClose,
  onCreated,
}: {
  userId: number;
  skus: PlatformService[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState<Partial<GrantSubscriptionReq>>({
    plan_kind: 'monthly',
    quota_total: 100000,
    qps_limit: 10,
  });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const skuOptions = skus.filter((s) => s.status === 'active' && s.is_public);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.sku_id) {
      setError('请选择 SKU');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await api.post(`/api/admin/users/${userId}/subscriptions`, {
        sku_id: form.sku_id,
        plan_kind: form.plan_kind ?? 'monthly',
        plan_name: form.plan_name,
        quota_total: form.quota_total ?? 100000,
        period_end: form.period_end,
        auto_renew: form.auto_renew ?? false,
        qps_limit: form.qps_limit ?? 10,
        daily_quota_limit: form.daily_quota_limit,
      } as GrantSubscriptionReq);
      onCreated();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="border-b border-ink-700 bg-ink-900/30 p-5">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <Select
          label="SKU *"
          value={form.sku_id ?? ''}
          onChange={(v) => setForm({ ...form, sku_id: v })}
          options={[
            { value: '', label: '— 请选择 —' },
            ...skuOptions.map((s) => ({ value: s.id, label: `${s.id} · ${s.display_name}` })),
          ]}
        />
        <Select
          label="套餐类型 *"
          value={form.plan_kind ?? 'monthly'}
          onChange={(v) => setForm({ ...form, plan_kind: v as GrantSubscriptionReq['plan_kind'] })}
          options={[
            { value: 'monthly', label: 'monthly · 包月（默认 30d）' },
            { value: 'prepaid', label: 'prepaid · 预付费（默认 90d）' },
            { value: 'trial', label: 'trial · 试用（默认 7d）' },
          ]}
        />
        <Field2
          label="套餐名（展示用）"
          value={form.plan_name ?? ''}
          onChange={(v) => setForm({ ...form, plan_name: v })}
          placeholder="DeepSeek Chat 标准版"
        />
        <NumberField2
          label="quota_total（按 SKU 计费单位）"
          value={form.quota_total}
          onChange={(v) => setForm({ ...form, quota_total: v })}
        />
        <NumberField2
          label="QPS 上限"
          value={form.qps_limit}
          onChange={(v) => setForm({ ...form, qps_limit: v })}
        />
        <NumberField2
          label="每日配额上限（可空）"
          value={form.daily_quota_limit}
          onChange={(v) => setForm({ ...form, daily_quota_limit: v })}
        />
        <Field2
          label="周期截止 (RFC3339, 可空)"
          value={form.period_end ?? ''}
          onChange={(v) => setForm({ ...form, period_end: v })}
          placeholder="2026-12-01T00:00:00Z"
        />
        <label className="flex items-center gap-2 text-sm text-ink-200">
          <input
            type="checkbox"
            checked={form.auto_renew ?? false}
            onChange={(e) => setForm({ ...form, auto_renew: e.target.checked })}
          />
          自动续订
        </label>
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
          disabled={submitting || !form.sku_id}
          className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
        >
          {submitting ? '创建中…' : '开订阅'}
        </button>
      </div>
    </form>
  );
}

function Field2({
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
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}

function NumberField2({
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
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <label className="block">
      <span className="text-xs text-ink-500">{label}</span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
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
