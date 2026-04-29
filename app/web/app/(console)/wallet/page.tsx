'use client';

import { useEffect, useState } from 'react';
import { api, type Recharge, type Wallet } from '@/lib/api';
import { fmtCents, fmtDateTime } from '@/lib/format';

export default function WalletPage() {
  const [wallet, setWallet] = useState<Wallet | null>(null);
  const [recharges, setRecharges] = useState<Recharge[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [amount, setAmount] = useState('100');
  const [submitting, setSubmitting] = useState(false);
  const [latestOrder, setLatestOrder] = useState<Recharge | null>(null);

  async function refresh() {
    try {
      const [w, r] = await Promise.all([
        api.get<Wallet>('/api/user/wallet'),
        api.get<{ data: Recharge[] }>('/api/user/wallet/recharges'),
      ]);
      setWallet(w);
      setRecharges(r.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const yuan = Number(amount);
    if (!Number.isFinite(yuan) || yuan <= 0) {
      setError('请输入正数金额');
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const order = await api.post<Recharge>('/api/user/wallet/recharge', {
        amount_cents: Math.round(yuan * 100),
        channel: 'manual',
      });
      setLatestOrder(order);
      await refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">钱包</h1>
        <p className="mt-1 text-sm text-ink-500">所有调用按毛利计费，余额不足将拒绝新请求。</p>
      </div>

      {wallet ? (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <Card label="可用余额" value={fmtCents(wallet.balance_cents, wallet.currency)} accent />
          <Card label="冻结金额" value={fmtCents(wallet.frozen_cents, wallet.currency)} />
          <Card label="累计消费" value={fmtCents(wallet.total_spent_cents, wallet.currency)} />
        </div>
      ) : (
        <div className="text-sm text-ink-500">加载中…</div>
      )}

      <form onSubmit={onCreate} className="rounded-2xl border border-ink-200 bg-white p-5">
        <div className="text-sm font-semibold">创建充值订单</div>
        <p className="mt-1 text-xs text-ink-500">
          MVP 阶段为人工对账模式：生成订单 → 付款 → 运营在 Admin 后台确认到账。
        </p>
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <label className="flex-1 min-w-[160px]">
            <span className="text-sm font-medium text-ink-700">金额（元）</span>
            <input
              type="number"
              min={1}
              step={1}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
            />
          </label>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white hover:bg-ink-800 disabled:opacity-60"
          >
            {submitting ? '生成中…' : '生成订单'}
          </button>
        </div>
        {latestOrder ? (
          <div className="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800">
            订单已生成 · 单号 <span className="mono">{latestOrder.order_no}</span> · 状态 {latestOrder.status}
          </div>
        ) : null}
      </form>

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-200 bg-white">
        <div className="border-b border-ink-200 px-5 py-3 text-sm font-medium">充值记录</div>
        <table className="w-full text-sm">
          <thead className="bg-ink-200/30 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-5 py-2.5 text-left">订单号</th>
              <th className="px-5 py-2.5 text-right">金额</th>
              <th className="px-5 py-2.5 text-left">渠道</th>
              <th className="px-5 py-2.5 text-left">状态</th>
              <th className="px-5 py-2.5 text-left">时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {recharges === null ? (
              <tr>
                <td colSpan={5} className="px-5 py-6 text-center text-ink-500">
                  加载中…
                </td>
              </tr>
            ) : recharges.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-12 text-center text-ink-500">
                  还没有充值记录。
                </td>
              </tr>
            ) : (
              recharges.map((r) => (
                <tr key={r.order_no}>
                  <td className="px-5 py-2.5 mono">{r.order_no}</td>
                  <td className="px-5 py-2.5 text-right">{fmtCents(r.amount_cents)}</td>
                  <td className="px-5 py-2.5">{r.channel}</td>
                  <td className="px-5 py-2.5">
                    <StatusPill status={r.status} />
                  </td>
                  <td className="px-5 py-2.5 text-ink-500">{fmtDateTime(r.created_at)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Card({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div
      className={`rounded-2xl border p-5 shadow-sm ${
        accent ? 'border-brand-500 bg-gradient-to-br from-white to-brand-50' : 'border-ink-200 bg-white'
      }`}
    >
      <div className="text-sm text-ink-500">{label}</div>
      <div className="mt-2 text-2xl font-semibold tracking-tight">{value}</div>
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone =
    status === 'paid'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'pending'
        ? 'bg-amber-100 text-amber-700'
        : status === 'cancelled' || status === 'failed'
          ? 'bg-rose-100 text-rose-700'
          : 'bg-ink-200 text-ink-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}
