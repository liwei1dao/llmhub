'use client';

import { useEffect, useState } from 'react';
import { api, type Vendor } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

// 厂商目录：代码常量来源（catalog.Vendors），admin 只读。
// 每张卡片把 vendor 自身元数据 + 它下面的业务板块（含凭据 schema 与
// 可绑能力）一次性铺出来——前端不需要再去拼别的接口。
export default function VendorsPage() {
  const [rows, setRows] = useState<Vendor[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: Vendor[] }>('/api/admin/catalog/vendors')
      .then((r) => setRows(sortByID(r.data ?? [])))
      .catch((err) => setError((err as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="厂商目录"
        subtitle="代码常量（internal/catalog/vendor.go）。每个厂商展开它下面的业务板块。"
      />

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {rows.map((v) => (
          <div key={v.id} className="rounded-2xl border border-ink-200 bg-white p-5">
            <div className="flex items-start justify-between">
              <div>
                <div className="text-base font-semibold text-ink-900">{v.name}</div>
                <div className="mt-0.5 text-xs text-ink-500 mono">{v.id}</div>
              </div>
              <span className="rounded-full bg-ink-100 px-2 py-0.5 text-[11px] text-ink-800">
                {(v.products ?? []).length} 板块
              </span>
            </div>
            {v.console_url ? (
              <div className="mt-2 text-[11px] text-ink-500">
                控制台：
                <a className="text-brand-200 hover:underline" href={v.console_url} target="_blank">
                  {v.console_url}
                </a>
              </div>
            ) : null}
            <div className="mt-3 text-[11px] text-ink-500">
              主账号鉴权字段：
              <span className="mono text-ink-800">
                {v.master_auth_schema.map((f) => f.key).join(' · ')}
              </span>
            </div>

            <div className="mt-4 space-y-2">
              {(v.products ?? []).map((p) => (
                <div key={p.id} className="rounded-lg border border-ink-200 bg-white p-3">
                  <div className="flex items-center justify-between">
                    <div className="text-sm font-medium text-ink-900">{p.name}</div>
                    <span className="text-[11px] text-ink-500 mono">{p.id}</span>
                  </div>
                  <div className="mt-1 text-[11px] text-ink-500">
                    凭据：
                    <span className="mono text-ink-800">
                      {p.credential_schema.map((f) => f.key).join(' · ')}
                    </span>
                  </div>
                  <div className="mt-1 text-[11px] text-ink-500">
                    可绑：
                    <span className="text-ink-800">{p.allowed_capabilities.join(' · ')}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function sortByID<T extends { id: string }>(xs: T[]): T[] {
  return [...xs].sort((a, b) => a.id.localeCompare(b.id));
}
