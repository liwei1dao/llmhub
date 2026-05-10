'use client';

import { useEffect, useState } from 'react';
import { api, type Capability, type PlatformCategory } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

// 上游能力字典：13 项常量按平台分类分组展示。
export default function CapabilitiesPage() {
  const [caps, setCaps] = useState<Capability[]>([]);
  const [cats, setCats] = useState<PlatformCategory[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.get<{ data: Capability[] }>('/api/admin/catalog/capabilities'),
      api.get<{ data: PlatformCategory[] }>('/api/admin/catalog/categories'),
    ])
      .then(([c, k]) => {
        setCaps(c.data ?? []);
        setCats(k.data ?? []);
      })
      .catch((err) => setError((err as Error).message));
  }, []);

  // group caps by category
  const byCat: Record<string, Capability[]> = {};
  for (const c of caps) {
    (byCat[c.category_id] ??= []).push(c);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="上游能力"
        subtitle="平台调度的最小单元。每条能力归属一个平台分类，每个业务板块声明它支持哪些能力。"
      />

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {cats.map((cat) => (
          <div key={cat.id} className="rounded-2xl border border-ink-200 bg-white p-5">
            <div className="flex items-center justify-between mb-3">
              <div className="text-base font-semibold">{cat.name}</div>
              <span className="text-xs text-ink-500 mono">{cat.id}</span>
            </div>
            <ul className="divide-y divide-ink-200 text-sm">
              {(byCat[cat.id] ?? []).map((c) => (
                <li key={c.id} className="flex items-center justify-between py-2">
                  <div>
                    <span className="mono text-ink-800">{c.id}</span>
                    <span className="text-ink-500"> · {c.display_name}</span>
                  </div>
                  <span className="text-xs text-ink-500">{c.billing_unit}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </div>
  );
}
