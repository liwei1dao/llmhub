'use client';

import Link from 'next/link';
import { use, useEffect, useState, type ReactNode } from 'react';
import {
  api,
  type Credential,
  type Vendor,
  type VendorAccount,
} from '@/lib/admin-api';
import { fmtCents, fmtDateTime } from '@/lib/format';
import PageHeader from '../../_components/page-header';

export default function AccountDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const acctID = Number(id);

  const [account, setAccount] = useState<VendorAccount | null>(null);
  const [vendor, setVendor] = useState<Vendor | null>(null);
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    setError(null);
    try {
      const [acctRes, credRes, vendorsRes] = await Promise.all([
        api.get<VendorAccount>(`/api/admin/vendor-accounts/${acctID}`),
        api.get<{ data: Credential[] }>(`/api/admin/credentials?account_id=${acctID}&limit=200`),
        api.get<{ data: Vendor[] }>(`/api/admin/catalog/vendors`),
      ]);
      setAccount(acctRes);
      setCredentials(credRes.data ?? []);
      const v = (vendorsRes.data ?? []).find((x) => x.id === acctRes.vendor_id);
      setVendor(v ?? null);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  useEffect(() => {
    if (acctID > 0) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [acctID]);

  async function archive() {
    if (!account) return;
    if (!confirm(`归档账号「${account.name}」？该账号的凭据将不再被调度。`)) return;
    setBusy(true);
    try {
      await api.delete(`/api/admin/vendor-accounts/${acctID}`);
      await load();
    } catch (err) {
      alert((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!account) {
    return (
      <div>
        <PageHeader title="账号详情" subtitle={`acct_${acctID}`} />
        {error ? <ErrorBox text={error} /> : <div className="text-ink-500">载入中...</div>}
      </div>
    );
  }

  const credActive = credentials.filter((c) => c.status === 'active').length;
  const credCooldown = credentials.filter((c) => c.status === 'cooldown').length;

  return (
    <div className="space-y-6">
      <PageHeader
        title={`🪪 ${account.name}`}
        subtitle={
          <span className="text-xs text-ink-500">
            <Link href="/accounts" className="hover:text-ink-800">账号管理</Link>
            <span className="mx-2 text-ink-700">/</span>
            <span className="mono">acct_{account.id}</span>
          </span>
        }
        actions={
          <button
            onClick={archive}
            disabled={busy || account.status === 'archived'}
            className="rounded-lg border border-rose-200 px-3 py-1.5 text-sm text-rose-700 hover:bg-rose-50 disabled:opacity-40"
          >
            归档账号
          </button>
        }
      />

      {error ? <ErrorBox text={error} /> : null}

      {/* 基本信息 */}
      <Section title="基本信息">
        <Grid>
          <Field label="厂商">
            {vendor ? `${vendor.name}` : account.vendor_id}
            <span className="ml-2 text-xs text-ink-500 mono">{account.vendor_id}</span>
          </Field>
          <Field label="状态"><StatusPill status={account.status} /></Field>
          <Field label="主体">{account.entity || <Dim>—</Dim>}</Field>
          <Field label="控制台">
            {account.console_url ? (
              <a className="text-brand-600 hover:underline" href={account.console_url} target="_blank" rel="noreferrer">
                {account.console_url}
              </a>
            ) : (
              <Dim>—</Dim>
            )}
          </Field>
          <Field label="创建">{fmtDateTime(account.created_at)}</Field>
          <Field label="最后更新">{fmtDateTime(account.updated_at)}</Field>
        </Grid>
      </Section>

      {/* 余额信息 */}
      <Section title="余额（最近一次抓取）">
        <Grid>
          <Field label="当前余额">
            {account.last_balance_cents != null ? (
              <span className="text-emerald-700 text-base font-medium">
                {fmtCents(account.last_balance_cents, account.last_balance_currency || 'CNY')}
              </span>
            ) : (
              <Dim>未抓取</Dim>
            )}
          </Field>
          <Field label="抓取时间">
            {account.last_balance_at ? fmtDateTime(account.last_balance_at) : <Dim>—</Dim>}
          </Field>
          <Field label="抓取错误">
            {account.last_balance_error ? (
              <span className="text-rose-700 text-xs">{account.last_balance_error}</span>
            ) : (
              <Dim>—</Dim>
            )}
          </Field>
        </Grid>
      </Section>

      {/* 关联凭据 */}
      <Section
        title="关联凭据"
        rightSlot={
          <div className="flex items-center gap-2 text-xs">
            <Pill tone="emerald">{credActive} 活</Pill>
            {credCooldown > 0 ? <Pill tone="amber">{credCooldown} 冷却</Pill> : null}
            <span className="text-ink-500">共 {credentials.length}</span>
            <Link
              href="/credentials/new"
              className="ml-2 rounded-lg bg-brand-600 px-3 py-1.5 font-medium text-white hover:bg-brand-700"
            >
              + 新增凭据
            </Link>
          </div>
        }
      >
        <div className="overflow-hidden rounded-xl border border-ink-200">
          <table className="w-full text-sm">
            <thead className="bg-ink-50 text-[11px] uppercase tracking-wider text-ink-500">
              <tr>
                <th className="px-4 py-2.5 text-left">名称</th>
                <th className="px-4 py-2.5 text-left">业务板块</th>
                <th className="px-4 py-2.5 text-left">环境</th>
                <th className="px-4 py-2.5 text-right">健康分</th>
                <th className="px-4 py-2.5 text-right">连续失败</th>
                <th className="px-4 py-2.5 text-left">最近使用</th>
                <th className="px-4 py-2.5 text-left">状态</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-200">
              {credentials.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-12 text-center text-ink-500">
                    暂无凭据 — 点击右上 + 新增凭据
                  </td>
                </tr>
              ) : (
                credentials.map((c) => (
                  <tr key={c.id} className="hover:bg-ink-50">
                    <td className="px-4 py-2.5">
                      <Link href={`/credentials/${c.id}`} className="text-ink-800 hover:text-brand-600">
                        🔑 {c.name}
                      </Link>
                      <div className="text-[11px] mono text-ink-500">cred_{c.id}</div>
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">{c.product_id}</td>
                    <td className="px-4 py-2.5 text-xs">{c.env}</td>
                    <td className="px-4 py-2.5 text-right">
                      <HealthBar score={c.health_score} />
                    </td>
                    <td className="px-4 py-2.5 text-right text-xs">
                      {c.consecutive_failures > 0 ? (
                        <span className="text-rose-700">{c.consecutive_failures}</span>
                      ) : (
                        <Dim>0</Dim>
                      )}
                    </td>
                    <td className="px-4 py-2.5 text-xs text-ink-500">
                      {c.last_used_at ? fmtDateTime(c.last_used_at) : <Dim>—</Dim>}
                    </td>
                    <td className="px-4 py-2.5">
                      <StatusPill status={c.status} />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Section>
    </div>
  );
}

// ---- 小组件 ----

function Section({
  title,
  rightSlot,
  children,
}: {
  title: string;
  rightSlot?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="rounded-2xl border border-ink-200 bg-white p-5">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-ink-800">{title}</h2>
        {rightSlot}
      </div>
      {children}
    </section>
  );
}

function Grid({ children }: { children: ReactNode }) {
  return <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{children}</div>;
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <div className="mb-0.5 text-[11px] uppercase tracking-wider text-ink-500">{label}</div>
      <div className="text-sm">{children}</div>
    </div>
  );
}

function Dim({ children }: { children: ReactNode }) {
  return <span className="text-ink-500">{children}</span>;
}

function ErrorBox({ text }: { text: string }) {
  return (
    <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
      {text}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const tone =
    status === 'active'
      ? 'bg-emerald-100 text-emerald-700'
      : status === 'cooldown' || status === 'frozen'
        ? 'bg-amber-100 text-amber-700'
        : 'bg-rose-100 text-rose-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-xs ${tone}`}>{status}</span>;
}

function Pill({ tone, children }: { tone: 'emerald' | 'amber'; children: ReactNode }) {
  const cls =
    tone === 'emerald'
      ? 'bg-emerald-100 text-emerald-700'
      : 'bg-amber-100 text-amber-700';
  return <span className={`inline-flex rounded-full px-2 py-0.5 ${cls}`}>{children}</span>;
}

function HealthBar({ score }: { score: number }) {
  const pct = Math.max(0, Math.min(100, score));
  const tone =
    pct >= 80 ? 'bg-emerald-500' : pct >= 50 ? 'bg-amber-500' : 'bg-rose-500';
  return (
    <div className="flex items-center justify-end gap-2">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-ink-100">
        <div className={`h-full ${tone}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs tabular-nums text-ink-500 w-7 text-right">{pct}</span>
    </div>
  );
}
