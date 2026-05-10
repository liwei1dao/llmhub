'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import {
  api,
  type Credential,
  type DashboardStats,
  type VendorAccount,
} from '@/lib/admin-api';
import PageHeader from '../_components/page-header';

// 总览：核心运营指标 + 最近变更。指标走单一 /api/admin/dashboard/stats 端点
// （后端在一次请求里跑多条 COUNT 查询），最近列表保持原来的分别拉取。
//
// 字段口径：
//   leases_active     = pool.leases status='active' AND expires_at > NOW()
//   subs_active       = iam.subscriptions status='active'
//   calls_today       = metering.call_logs ts >= 今日 UTC 0 点
//   recharges_pending = wallet.recharges status='pending' （需操作员确认）
export default function Dashboard() {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [accounts, setAccounts] = useState<VendorAccount[]>([]);
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      api.get<DashboardStats>('/api/admin/dashboard/stats'),
      api.get<{ data: VendorAccount[] }>('/api/admin/vendor-accounts?limit=8'),
      api.get<{ data: Credential[] }>('/api/admin/credentials?limit=8'),
    ])
      .then(([s, a, c]) => {
        setStats(s);
        setAccounts(a.data ?? []);
        setCredentials(c.data ?? []);
      })
      .catch((err) => setError((err as Error).message));
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="运营总览"
        subtitle="聚合 SDK 平台日常关键指标 — 凭据池 / 用户订阅 / 活 lease / 今日调用"
      />

      {error ? (
        <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {error}
        </div>
      ) : null}

      {/* 第一行：用户侧指标 — 用户、活 API key、活订阅、活 lease */}
      <Section title="用户与 SDK">
        <Stat label="注册用户" value={stats?.users_total} hint="iam.users" />
        <Stat label="活 API Key" value={stats?.api_keys_active} hint="status=active" accent="emerald" />
        <Stat
          label="活订阅"
          value={stats?.subscriptions_active}
          sub={stats ? `共 ${stats.subscriptions_total ?? 0}` : undefined}
          hint="iam.subscriptions"
          accent="emerald"
        />
        <Stat
          label="活 Lease"
          value={stats?.leases_active}
          sub={stats ? `累计签 ${stats.leases_total ?? 0}` : undefined}
          hint="未过期"
          accent="brand"
        />
      </Section>

      {/* 第二行：上游 + 调用量 + 待办 */}
      <Section title="上游池 + 业务">
        <Stat label="凭据 (active)" value={stats?.credentials_active} accent="emerald" />
        <Stat
          label="凭据 (cooldown)"
          value={stats?.credentials_cooldown}
          accent="amber"
          hint="健康分恢复中"
        />
        <Stat
          label="今日调用"
          value={stats?.calls_today}
          sub={stats ? `成功 ${stats.calls_success_today ?? 0}` : undefined}
          hint="metering.call_logs"
        />
        <Stat
          label="待确认充值"
          value={stats?.recharges_pending}
          accent={stats?.recharges_pending && stats.recharges_pending > 0 ? 'amber' : undefined}
          hint="需 admin 处理"
        />
      </Section>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RecentTable
          title="最近主账号"
          href="/accounts"
          empty={
            <>
              暂无主账号 — 去 <Link href="/accounts" className="underline">主账号管理</Link> 创建
            </>
          }
          headers={['名称', '厂商', '主体', '状态']}
          rows={accounts.map((a) => [
            <span key="n">🪪 {a.name}</span>,
            <span key="v" className="text-xs text-ink-500">{a.vendor_id}</span>,
            <span key="e" className="text-xs">{a.entity ?? '—'}</span>,
            <span key="s" className="text-xs">{a.status}</span>,
          ])}
        />
        <RecentTable
          title="最近凭据"
          href="/credentials"
          empty={
            <>
              暂无凭据 — 去 <Link href="/credentials/new" className="underline">新增凭据</Link>
            </>
          }
          headers={['名称', '板块', '健康', '状态']}
          rows={credentials.map((c) => [
            <span key="n">🔑 {c.name}</span>,
            <span key="p" className="mono text-xs">{c.product_id}</span>,
            <span key="h" className="text-right">{c.health_score}</span>,
            <span key="s" className="text-xs">{c.status}</span>,
          ])}
        />
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-2 text-[11px] uppercase tracking-wider text-ink-500">{title}</div>
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">{children}</div>
    </div>
  );
}

function Stat({
  label,
  value,
  sub,
  hint,
  accent,
}: {
  label: string;
  value: number | undefined;
  sub?: string;
  hint?: string;
  accent?: 'emerald' | 'amber' | 'rose' | 'brand';
}) {
  const tone =
    accent === 'emerald'
      ? 'text-emerald-700'
      : accent === 'amber'
        ? 'text-amber-700'
        : accent === 'rose'
          ? 'text-rose-700'
          : accent === 'brand'
            ? 'text-brand-600'
            : 'text-ink-900';
  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="text-sm text-ink-500">{label}</div>
      <div className={`mt-2 text-3xl font-semibold tracking-tight ${tone}`}>
        {value === undefined ? <span className="text-ink-500">—</span> : value.toLocaleString()}
      </div>
      {sub ? <div className="mt-1 text-[11px] text-ink-500">{sub}</div> : null}
      {hint ? <div className="mt-0.5 text-[10px] text-ink-500">{hint}</div> : null}
    </div>
  );
}

function RecentTable({
  title,
  href,
  headers,
  rows,
  empty,
}: {
  title: string;
  href: string;
  headers: string[];
  rows: React.ReactNode[][];
  empty: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-ink-200 bg-white">
      <div className="flex items-center justify-between border-b border-ink-200 px-5 py-3 text-sm font-medium text-ink-900">
        <span>{title}</span>
        <Link href={href} className="text-xs text-brand-200 hover:underline">
          查看全部 →
        </Link>
      </div>
      <table className="w-full text-sm">
        <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
          <tr>
            {headers.map((h) => (
              <th key={h} className="px-5 py-2 text-left">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-ink-200">
          {rows.length === 0 ? (
            <tr>
              <td colSpan={headers.length} className="px-5 py-8 text-center text-ink-500">
                {empty}
              </td>
            </tr>
          ) : (
            rows.map((cells, i) => (
              <tr key={i}>
                {cells.map((c, j) => (
                  <td key={j} className="px-5 py-2">
                    {c}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
