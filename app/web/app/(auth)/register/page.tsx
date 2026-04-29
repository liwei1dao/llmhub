'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { api } from '@/lib/api';

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      await api.post('/api/user/auth/register', {
        email,
        password,
        display_name: displayName,
      });
      await api.post('/api/user/auth/login', { email, password });
      router.push('/dashboard');
      router.refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-8 shadow-sm">
      <h1 className="text-2xl font-bold tracking-tight">创建 LLMHub 账户</h1>
      <p className="mt-1 text-sm text-ink-500">注册即送测试额度，几秒钟内开始第一次调用。</p>

      <form className="mt-6 space-y-4" onSubmit={onSubmit}>
        <Field label="邮箱" type="email" value={email} onChange={setEmail} autoComplete="email" />
        <Field
          label="昵称（可选）"
          type="text"
          value={displayName}
          onChange={setDisplayName}
          autoComplete="name"
          required={false}
        />
        <Field
          label="密码"
          type="password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          hint="至少 8 位"
        />
        {error ? (
          <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </div>
        ) : null}
        <button
          type="submit"
          disabled={loading}
          className="w-full rounded-lg bg-ink-900 py-2.5 text-sm font-medium text-white hover:bg-ink-800 disabled:opacity-60"
        >
          {loading ? '注册中…' : '注册并登录'}
        </button>
      </form>

      <div className="mt-6 text-sm text-ink-500">
        已有账户?{' '}
        <Link href="/login" className="font-medium text-brand-600 hover:underline">
          直接登录
        </Link>
      </div>
    </div>
  );
}

function Field({
  label,
  type,
  value,
  onChange,
  autoComplete,
  required = true,
  hint,
}: {
  label: string;
  type: string;
  value: string;
  onChange: (v: string) => void;
  autoComplete?: string;
  required?: boolean;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="text-sm font-medium text-ink-700">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        autoComplete={autoComplete}
        className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none ring-brand-500/30 transition focus:border-brand-500 focus:ring-2"
      />
      {hint ? <span className="mt-1 block text-xs text-ink-500">{hint}</span> : null}
    </label>
  );
}
