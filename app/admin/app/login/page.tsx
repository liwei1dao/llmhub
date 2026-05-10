'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { getToken, login } from '@/lib/admin-api';

export default function AdminLogin() {
  const router = useRouter();
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (getToken()) router.replace('/dashboard');
  }, [router]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!account.trim() || !password) {
      setError('账号和密码不能为空');
      return;
    }
    setSubmitting(true);
    try {
      await login(account.trim(), password);
      router.push('/dashboard');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center px-6">
      <div className="w-full max-w-md rounded-2xl border border-ink-200 bg-white p-8 shadow-xl">
        <div className="mb-6 flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-gradient-to-br from-rose-500 to-orange-500 font-semibold text-white">
            A
          </span>
          <span className="text-xl font-semibold tracking-tight text-ink-900">LLMHub Admin</span>
        </div>
        <h1 className="text-lg font-semibold text-ink-900">后台登录</h1>
        <p className="mt-1 text-sm text-ink-500">
          管理员账号独立于终端用户。账号由首位 admin 通过 <code className="mono rounded bg-ink-100 px-1">LLMHUB_ADMIN_BOOTSTRAP_*</code> 环境变量种子化，后续可在后台扩展。
        </p>
        <form className="mt-5 space-y-4" onSubmit={onSubmit}>
          <label className="block">
            <span className="text-xs text-ink-500">登录账号</span>
            <input
              type="text"
              autoComplete="username"
              value={account}
              onChange={(e) => setAccount(e.target.value)}
              placeholder="管理员账号"
              className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-500">登录密码</span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="密码"
              className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
            />
          </label>
          {error ? <div className="text-sm text-rose-700">{error}</div> : null}
          <button
            type="submit"
            disabled={submitting}
            className="w-full rounded-lg bg-brand-600 py-2.5 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60"
          >
            {submitting ? '登录中…' : '登录'}
          </button>
        </form>
      </div>
    </div>
  );
}
