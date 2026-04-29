'use client';

import { useEffect, useState } from 'react';
import { api, type UsageBucket } from '@/lib/api';
import { fmtCents, fmtNumber } from '@/lib/format';

const RANGES: Array<'1d' | '7d' | '30d'> = ['1d', '7d', '30d'];

export default function UsagePage() {
  const [range, setRange] = useState<'1d' | '7d' | '30d'>('7d');
  const [data, setData] = useState<UsageBucket[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    api
      .get<{ data: UsageBucket[] }>(`/api/user/usage/series?range=${range}`)
      .then((res) => setData(res.data ?? []))
      .catch((err) => setError((err as Error).message))
      .finally(() => setLoading(false));
  }, [range]);

  const byCapability = data.reduce<Record<string, { calls: number; cost: number }>>((acc, b) => {
    if (!acc[b.capability_id]) acc[b.capability_id] = { calls: 0, cost: 0 };
    acc[b.capability_id].calls += b.calls;
    acc[b.capability_id].cost += b.cost_retail_cents;
    return acc;
  }, {});

  return (
    <div className="space-y-6">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">用量</h1>
          <p className="mt-1 text-sm text-ink-500">按日聚合，按能力域分组。</p>
        </div>
        <div className="inline-flex rounded-lg border border-ink-200 bg-white p-1 text-sm">
          {RANGES.map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`rounded-md px-3 py-1 transition ${
                r === range ? 'bg-ink-900 text-white' : 'text-ink-700 hover:bg-ink-200/40'
              }`}
            >
              {r === '1d' ? '昨日' : r === '7d' ? '7 天' : '30 天'}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {Object.entries(byCapability).length === 0 ? (
          <div className="md:col-span-3 rounded-2xl border border-dashed border-ink-200 bg-white p-12 text-center text-sm text-ink-500">
            该时间段内无用量。
          </div>
        ) : (
          Object.entries(byCapability).map(([cap, agg]) => (
            <div key={cap} className="rounded-2xl border border-ink-200 bg-white p-5">
              <div className="text-sm text-ink-500">{cap}</div>
              <div className="mt-2 text-2xl font-semibold">{fmtNumber(agg.calls)}</div>
              <div className="mt-1 text-xs text-ink-500">合计 {fmtCents(agg.cost)}</div>
            </div>
          ))
        )}
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-200 bg-white">
        <table className="w-full text-sm">
          <thead className="bg-ink-200/30 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-5 py-2.5 text-left">日期</th>
              <th className="px-5 py-2.5 text-left">能力</th>
              <th className="px-5 py-2.5 text-right">调用</th>
              <th className="px-5 py-2.5 text-right">成功</th>
              <th className="px-5 py-2.5 text-right">Token In</th>
              <th className="px-5 py-2.5 text-right">Token Out</th>
              <th className="px-5 py-2.5 text-right">音频秒</th>
              <th className="px-5 py-2.5 text-right">字符</th>
              <th className="px-5 py-2.5 text-right">消费</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {loading ? (
              <tr>
                <td colSpan={9} className="px-5 py-6 text-center text-ink-500">
                  加载中…
                </td>
              </tr>
            ) : data.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-5 py-12 text-center text-ink-500">
                  无数据
                </td>
              </tr>
            ) : (
              data.map((b) => (
                <tr key={`${b.day}-${b.capability_id}`}>
                  <td className="px-5 py-2.5 mono text-ink-700">{b.day}</td>
                  <td className="px-5 py-2.5">{b.capability_id}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.calls)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.success_calls)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.tokens_in)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.tokens_out)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.audio_seconds)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtNumber(b.characters)}</td>
                  <td className="px-5 py-2.5 text-right">{fmtCents(b.cost_retail_cents)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
