'use client';

import { useEffect, useState } from 'react';
import { api, type VendorProduct } from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

// 业务板块字典：12 项代码常量平铺（internal/catalog/product.go）。
// 加新板块需要写 adapter，所以这一页只读。
export default function ProductsPage() {
  const [rows, setRows] = useState<VendorProduct[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .get<{ data: VendorProduct[] }>('/api/admin/catalog/products')
      .then((r) => setRows([...(r.data ?? [])].sort((a, b) => a.id.localeCompare(b.id))))
      .catch((err) => setError((err as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="业务板块"
        subtitle="代码常量。每个板块声明自己的凭据 schema、协议族与可绑能力。"
      />

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-700 bg-ink-800">
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">板块 ID</th>
              <th className="px-4 py-2.5 text-left">中文名</th>
              <th className="px-4 py-2.5 text-left">厂商</th>
              <th className="px-4 py-2.5 text-left">凭据字段</th>
              <th className="px-4 py-2.5 text-left">可绑能力</th>
              <th className="px-4 py-2.5 text-left">协议</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.map((p) => (
              <tr key={p.id}>
                <td className="px-4 py-2.5 mono text-ink-200">{p.id}</td>
                <td className="px-4 py-2.5">{p.name}</td>
                <td className="px-4 py-2.5 text-ink-500">{p.vendor_id}</td>
                <td className="px-4 py-2.5 text-xs text-ink-500 mono">
                  {p.credential_schema.map((f) => f.key).join(', ')}
                </td>
                <td className="px-4 py-2.5 text-xs text-ink-500">
                  {p.allowed_capabilities.join(' · ')}
                </td>
                <td className="px-4 py-2.5 text-xs text-ink-500">{p.protocol_family}</td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-5 py-12 text-center text-ink-500">
                  无数据
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
