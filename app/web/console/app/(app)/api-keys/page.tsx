'use client';

import { useEffect, useState } from 'react';
import { api, type APIKey, type CreatedAPIKey } from '@/lib/api';
import { fmtDateTime } from '@/lib/format';

export default function APIKeysPage() {
  const [keys, setKeys] = useState<APIKey[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [creating, setCreating] = useState(false);
  const [revealed, setRevealed] = useState<CreatedAPIKey | null>(null);

  async function load() {
    try {
      const res = await api.get<{ data: APIKey[] }>('/api/user/api-keys');
      setKeys(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    setError(null);
    try {
      const res = await api.post<CreatedAPIKey>('/api/user/api-keys', {
        name: name || 'default',
        scopes: ['chat:read', 'chat:write'],
      });
      setRevealed(res);
      setName('');
      await load();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">API Key</h1>
        <p className="mt-1 text-sm text-ink-500">在请求头里携带 <code className="mono rounded bg-ink-200/50 px-1">Authorization: Bearer &lt;key&gt;</code>。</p>
      </div>

      {revealed ? (
        <div className="rounded-2xl border border-amber-300 bg-amber-50 p-5">
          <div className="text-sm font-semibold text-amber-900">key 仅显示一次，请妥善保存：</div>
          <div className="mono mt-2 break-all rounded-lg bg-white px-3 py-2 text-sm text-amber-900">
            {revealed.key}
          </div>
          <button
            onClick={() => setRevealed(null)}
            className="mt-3 text-sm text-amber-900 underline"
          >
            我已复制
          </button>
        </div>
      ) : null}

      <form onSubmit={onCreate} className="flex items-end gap-3 rounded-2xl border border-ink-200 bg-white p-5">
        <label className="flex-1">
          <span className="text-sm font-medium text-ink-700">名称</span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. prod-app / local-dev"
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
          />
        </label>
        <button
          type="submit"
          disabled={creating}
          className="rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white hover:bg-ink-800 disabled:opacity-60"
        >
          {creating ? '创建中…' : '创建新 Key'}
        </button>
      </form>

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-200 bg-white">
        <table className="w-full text-sm">
          <thead className="bg-ink-200/30 text-xs uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-5 py-2.5 text-left">名称</th>
              <th className="px-5 py-2.5 text-left">前缀</th>
              <th className="px-5 py-2.5 text-left">权限</th>
              <th className="px-5 py-2.5 text-left">状态</th>
              <th className="px-5 py-2.5 text-left">创建时间</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-200">
            {keys === null ? (
              <tr>
                <td colSpan={5} className="px-5 py-6 text-center text-ink-500">
                  加载中…
                </td>
              </tr>
            ) : keys.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-12 text-center text-ink-500">
                  还没有 Key —— 用上方表单创建一个。
                </td>
              </tr>
            ) : (
              keys.map((k) => (
                <tr key={k.id}>
                  <td className="px-5 py-2.5">{k.name ?? '-'}</td>
                  <td className="px-5 py-2.5 mono">{k.prefix}…</td>
                  <td className="px-5 py-2.5 text-ink-500">{(k.scopes ?? []).join(', ') || '-'}</td>
                  <td className="px-5 py-2.5">
                    <StatusPill status={k.status} />
                  </td>
                  <td className="px-5 py-2.5 text-ink-500">{fmtDateTime(k.created)}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'revoked'
        ? 'bg-rose-100 text-rose-700'
        : 'bg-ink-200 text-ink-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}
