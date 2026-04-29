'use client';

import { useState } from 'react';
import { api } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

export default function RechargesPage() {
  const [orderNo, setOrderNo] = useState('');
  const [channelOrderID, setChannelOrderID] = useState('');
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function confirm(e: React.FormEvent) {
    e.preventDefault();
    if (!orderNo.trim()) {
      setError('请输入订单号');
      return;
    }
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      await api.post(`/api/admin/recharges/${encodeURIComponent(orderNo.trim())}/confirm`, {
        channel_order_id: channelOrderID,
      });
      setResult(`订单 ${orderNo} 已确认到账`);
      setOrderNo('');
      setChannelOrderID('');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="充值确认"
        subtitle="对人工对账渠道，运营粘贴订单号 + 渠道流水号确认到账（idempotent）"
      />

      <form onSubmit={confirm} className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <label className="block">
            <span className="text-xs text-ink-500">order_no（用户钱包页生成）</span>
            <input
              value={orderNo}
              onChange={(e) => setOrderNo(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
              placeholder="rg_xxxxxxxxxx"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-500">channel_order_id（渠道流水号 / 备注）</span>
            <input
              value={channelOrderID}
              onChange={(e) => setChannelOrderID(e.target.value)}
              className="mt-1 block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500"
            />
          </label>
        </div>

        <div className="mt-4 flex justify-end">
          <button
            type="submit"
            disabled={loading}
            className="rounded-lg bg-white px-4 py-2 text-sm font-medium text-ink-900 hover:bg-ink-200 disabled:opacity-60"
          >
            {loading ? '确认中…' : '确认到账'}
          </button>
        </div>

        {result ? (
          <div className="mt-4 rounded-lg border border-emerald-700 bg-emerald-950 px-3 py-2 text-sm text-emerald-200">
            {result}
          </div>
        ) : null}
        {error ? (
          <div className="mt-4 rounded-lg border border-rose-700 bg-rose-950 px-3 py-2 text-sm text-rose-200">
            {error}
          </div>
        ) : null}
      </form>
    </div>
  );
}
