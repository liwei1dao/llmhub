'use client';

import { useEffect, useState } from 'react';
import { api, type ProviderRow } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

export default function ProvidersPage() {
  const [rows, setRows] = useState<ProviderRow[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: ProviderRow[] }>('/api/admin/providers')
      .then((r) => setRows(r.data ?? []))
      .catch((err) => setError((err as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="厂商"
        subtitle="catalog.providers 表 — 由 configs/providers/*.yaml reconcile 生成"
      />

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {rows.length === 0 ? (
          <div className="md:col-span-3 rounded-2xl border border-dashed border-ink-700 bg-ink-800 p-12 text-center text-sm text-ink-500">
            暂无厂商记录
          </div>
        ) : (
          rows.map((p) => (
            <div key={p.id} className="rounded-2xl border border-ink-700 bg-ink-800 p-5">
              <div className="flex items-center justify-between">
                <div className="text-base font-semibold text-white">{p.display_name}</div>
                <span className="rounded-full bg-ink-700 px-2 py-0.5 text-xs text-ink-200">
                  {p.status}
                </span>
              </div>
              <div className="mt-1 text-xs text-ink-500 mono">{p.id}</div>
              <div className="mt-3 text-xs text-ink-500">
                <div>protocol: {p.protocol_family}</div>
                <div className="truncate">base_url: {p.base_url}</div>
              </div>
              <div className="mt-4 flex flex-wrap gap-1.5 text-xs">
                {(p.capabilities ?? []).map((c) => (
                  <span key={c} className="rounded-full bg-brand-500/20 px-2 py-0.5 text-brand-200">
                    {c}
                  </span>
                ))}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
