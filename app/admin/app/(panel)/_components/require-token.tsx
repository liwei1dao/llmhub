'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState, type ReactNode } from 'react';
import { getToken, me, setToken } from '@/lib/admin-api';

// RequireToken 在 (panel) 组前面把"未登录 / 会话已过期"踢回 /login。
// 1) 本地无 token → 立即跳转
// 2) 有 token 但服务端 /auth/me 返回非 2xx → 清掉 token + 跳转
// 这一步用 /me 校验，避免 token 在数据库里已被吊销但 localStorage 还留着。
export default function RequireToken({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let cancelled = false;
    if (!getToken()) {
      router.replace('/login');
      return;
    }
    me()
      .then(() => {
        if (!cancelled) setReady(true);
      })
      .catch(() => {
        if (cancelled) return;
        setToken('');
        router.replace('/login');
      });
    return () => {
      cancelled = true;
    };
  }, [router]);

  if (!ready) return null;
  return <>{children}</>;
}
