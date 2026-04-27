'use client';

import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';

export default function LogoutButton() {
  const router = useRouter();
  async function logout() {
    try {
      await api.post('/api/user/auth/logout');
    } catch {
      // best-effort — even on failure clear the local route
    }
    router.push('/login');
    router.refresh();
  }
  return (
    <button
      onClick={logout}
      className="w-full rounded-lg px-3 py-2 text-left text-sm text-ink-500 transition hover:bg-ink-200/40 hover:text-ink-800"
    >
      退出登录
    </button>
  );
}
