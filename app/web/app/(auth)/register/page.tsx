'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';

export default function RegisterPage() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // 验证码发送状态：sending + 60s cooldown + 提示是否真发出去了
  const [sending, setSending] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const cooldownTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  useEffect(() => {
    return () => {
      if (cooldownTimer.current) clearInterval(cooldownTimer.current);
    };
  }, []);

  function startCooldown(seconds: number) {
    setCooldown(seconds);
    if (cooldownTimer.current) clearInterval(cooldownTimer.current);
    cooldownTimer.current = setInterval(() => {
      setCooldown((c) => {
        if (c <= 1) {
          if (cooldownTimer.current) clearInterval(cooldownTimer.current);
          return 0;
        }
        return c - 1;
      });
    }, 1000);
  }

  async function onSendCode() {
    setError(null);
    setInfo(null);
    if (!email) {
      setError('请先填邮箱');
      return;
    }
    setSending(true);
    try {
      const res = await api.post<{ delivered: boolean; cooldown_seconds: number }>(
        '/api/user/auth/send-code',
        { email },
      );
      startCooldown(res.cooldown_seconds || 60);
      setInfo(
        res.delivered
          ? '验证码已发送，请查收邮件（也检查垃圾邮件箱）。'
          : '邮件未配置，验证码已写入服务端日志（dev 模式）。',
      );
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSending(false);
    }
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setInfo(null);
    setSubmitting(true);
    try {
      await api.post('/api/user/auth/register', {
        email,
        code,
        password,
        display_name: displayName,
      });
      await api.post('/api/user/auth/login', { email, password });
      router.push('/dashboard');
      router.refresh();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="rounded-2xl border border-ink-200 bg-white p-8 shadow-sm">
      <h1 className="text-2xl font-bold tracking-tight">创建 LLMHub 账户</h1>
      <p className="mt-1 text-sm text-ink-500">注册即送测试额度，几秒钟内开始第一次调用。</p>

      <form className="mt-6 space-y-4" onSubmit={onSubmit}>
        <label className="block">
          <span className="text-sm font-medium text-ink-700">邮箱</span>
          <div className="mt-1 flex gap-2">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="flex-1 rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none ring-brand-500/30 transition focus:border-brand-500 focus:ring-2"
            />
            <button
              type="button"
              onClick={onSendCode}
              disabled={sending || cooldown > 0}
              className="shrink-0 rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm font-medium text-ink-800 transition hover:bg-ink-50 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {sending ? '发送中…' : cooldown > 0 ? `${cooldown} s` : '发送验证码'}
            </button>
          </div>
        </label>

        <label className="block">
          <span className="text-sm font-medium text-ink-700">邮箱验证码</span>
          <input
            type="text"
            inputMode="numeric"
            pattern="\d{6}"
            maxLength={6}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
            required
            autoComplete="one-time-code"
            placeholder="6 位数字"
            className="mono mt-1 block w-full tracking-[0.4em] rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
          />
        </label>

        <label className="block">
          <span className="text-sm font-medium text-ink-700">昵称（可选）</span>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            autoComplete="name"
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
          />
        </label>

        <label className="block">
          <span className="text-sm font-medium text-ink-700">密码</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoComplete="new-password"
            className="mt-1 block w-full rounded-lg border border-ink-200 bg-white px-3 py-2 text-sm shadow-sm outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/30"
          />
          <span className="mt-1 block text-xs text-ink-500">至少 8 位</span>
        </label>

        {error ? (
          <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </div>
        ) : null}
        {info ? (
          <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {info}
          </div>
        ) : null}

        <button
          type="submit"
          disabled={submitting}
          className="w-full rounded-lg bg-ink-900 py-2.5 text-sm font-medium text-white hover:bg-ink-800 disabled:opacity-60"
        >
          {submitting ? '注册中…' : '注册并登录'}
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
