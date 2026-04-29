'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { getToken, setToken } from '@/lib/admin-api';

export default function AdminLogin() {
  const router = useRouter();
  const [token, setLocal] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (getToken()) router.replace('/admin/dashboard');
  }, [router]);

  function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!token.trim()) {
      setError('请输入 LLMHUB_ADMIN_TOKEN');
      return;
    }
    setToken(token.trim());
    router.push('/admin/dashboard');
  }

  return (
    <div className="grid min-h-screen place-items-center px-6">
      <div className="w-full max-w-md rounded-2xl border border-ink-700 bg-ink-800 p-8 shadow-xl">
        <div className="mb-6 flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-gradient-to-br from-rose-500 to-orange-500 font-semibold text-white">
            A
          </span>
          <span className="text-xl font-semibold tracking-tight text-white">LLMHub Admin</span>
        </div>
        <h1 className="text-lg font-semibold text-white">输入管理员 Token</h1>
        <p className="mt-1 text-sm text-ink-500">
          来自 account 服务环境变量 <code className="mono rounded bg-ink-900 px-1">LLMHUB_ADMIN_TOKEN</code>，
          浏览器本地存储后随每次请求发送 <code className="mono">X-Admin-Token</code>。
        </p>
        <form className="mt-5 space-y-4" onSubmit={onSubmit}>
          <input
            type="password"
            value={token}
            onChange={(e) => setLocal(e.target.value)}
            placeholder="LLMHUB_ADMIN_TOKEN"
            className="block w-full rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-white outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
          />
          {error ? <div className="text-sm text-rose-400">{error}</div> : null}
          <button
            type="submit"
            className="w-full rounded-lg bg-white py-2.5 text-sm font-semibold text-ink-900 hover:bg-ink-200"
          >
            进入后台
          </button>
        </form>
      </div>
    </div>
  );
}
