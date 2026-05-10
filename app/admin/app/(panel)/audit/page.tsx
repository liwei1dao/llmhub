'use client';

import { useEffect, useState } from 'react';
import { api, type AuditLog } from '@/lib/admin-api';
import { fmtDateTime } from '@/lib/format';
import PageHeader from '../_components/page-header';

// 审计日志查看页：按 actor / action / target 过滤，看每条 admin 操作
// 的痕迹。payload 字段是 admin handler 写入的小结构化上下文（订阅 id、
// 改价前后值等），这里做 JSON 折叠展示，避免把表格挤爆。
export default function AuditPage() {
  const [rows, setRows] = useState<AuditLog[]>([]);
  const [filter, setFilter] = useState({ action: '', target_type: '', target_id: '' });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      if (filter.action) params.set('action', filter.action);
      if (filter.target_type) params.set('target_type', filter.target_type);
      if (filter.target_id) params.set('target_id', filter.target_id);
      params.set('limit', '300');
      const res = await api.get<{ data: AuditLog[] }>(`/api/admin/audit-logs?${params}`);
      setRows(res.data ?? []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="space-y-6">
      <PageHeader
        title="审计日志"
        subtitle="所有管理员操作的留痕。按 action / target 过滤，便于事故追溯。"
      />

      <div className="flex flex-wrap gap-3 rounded-2xl border border-ink-700 bg-ink-800 p-4 text-sm">
        <FilterInput
          label="action"
          value={filter.action}
          onChange={(v) => setFilter({ ...filter, action: v })}
          placeholder="grant_subscription / revoke_lease …"
        />
        <FilterInput
          label="target_type"
          value={filter.target_type}
          onChange={(v) => setFilter({ ...filter, target_type: v })}
          placeholder="subscription / lease / platform_service"
        />
        <FilterInput
          label="target_id"
          value={filter.target_id}
          onChange={(v) => setFilter({ ...filter, target_id: v })}
          placeholder="目标 id"
        />
        <button
          onClick={load}
          disabled={loading}
          className="self-end rounded-lg bg-brand-600 px-4 py-2 font-medium text-white hover:bg-brand-700 disabled:opacity-60"
        >
          {loading ? '查询中…' : '查询'}
        </button>
      </div>

      {error ? (
        <div className="rounded-lg border border-rose-700 bg-rose-950 px-4 py-3 text-sm text-rose-200">
          {error}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-2xl border border-ink-700 bg-ink-800">
        <table className="w-full text-sm">
          <thead className="bg-ink-900/40 text-[11px] uppercase tracking-wider text-ink-500">
            <tr>
              <th className="px-4 py-2.5 text-left">时间</th>
              <th className="px-4 py-2.5 text-left">actor</th>
              <th className="px-4 py-2.5 text-left">action</th>
              <th className="px-4 py-2.5 text-left">target</th>
              <th className="px-4 py-2.5 text-left">结果</th>
              <th className="px-4 py-2.5 text-left">payload</th>
              <th className="px-4 py-2.5 text-left">IP</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-ink-700">
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-5 py-12 text-center text-ink-500">
                  无审计行
                </td>
              </tr>
            ) : (
              rows.map((row) => <AuditRow key={row.id} row={row} />)
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function AuditRow({ row }: { row: AuditLog }) {
  const [open, setOpen] = useState(false);
  const hasPayload = row.payload !== undefined && row.payload !== null;
  return (
    <>
      <tr>
        <td className="px-4 py-2.5 text-xs text-ink-500">{fmtDateTime(row.created_at)}</td>
        <td className="px-4 py-2.5 text-xs">
          <div className="text-ink-200">{row.actor_type}</div>
          {row.actor_id ? <div className="mono text-[10px] text-ink-500">#{row.actor_id}</div> : null}
        </td>
        <td className="px-4 py-2.5 mono text-xs text-ink-200">{row.action}</td>
        <td className="px-4 py-2.5 mono text-xs">
          {row.target_type ? (
            <>
              <div className="text-ink-200">{row.target_type}</div>
              <div className="text-[10px] text-ink-500">{row.target_id}</div>
            </>
          ) : (
            <span className="text-ink-500">—</span>
          )}
        </td>
        <td className="px-4 py-2.5">
          <ResultPill result={row.result} />
        </td>
        <td className="px-4 py-2.5">
          {hasPayload ? (
            <button
              onClick={() => setOpen((v) => !v)}
              className="text-xs text-brand-400 hover:underline"
            >
              {open ? '收起' : '展开'}
            </button>
          ) : (
            <span className="text-xs text-ink-500">—</span>
          )}
        </td>
        <td className="px-4 py-2.5 mono text-[11px] text-ink-500">{row.ip || '—'}</td>
      </tr>
      {open && hasPayload ? (
        <tr>
          <td colSpan={7} className="bg-ink-900/40 px-4 py-2.5">
            <pre className="mono overflow-x-auto whitespace-pre-wrap break-all text-[11px] text-ink-300">
              {JSON.stringify(row.payload, null, 2)}
            </pre>
          </td>
        </tr>
      ) : null}
    </>
  );
}

function FilterInput({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <label className="flex flex-col">
      <span className="text-xs text-ink-500">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 rounded-lg border border-ink-700 bg-ink-900 px-3 py-1.5 text-sm text-white outline-none focus:border-brand-500"
      />
    </label>
  );
}

function ResultPill({ result }: { result: string }) {
  const tone =
    result === 'ok'
      ? 'bg-emerald-500/20 text-emerald-300'
      : result === 'error'
        ? 'bg-rose-500/20 text-rose-300'
        : result === 'denied'
          ? 'bg-amber-500/20 text-amber-300'
          : 'bg-ink-700 text-ink-200';
  return <span className={`inline-flex rounded-full px-2 py-0.5 text-[11px] ${tone}`}>{result || '—'}</span>;
}
