'use client';

import { useRouter } from 'next/navigation';
import { setToken } from '@/lib/admin-api';

export default function LogoutButton() {
  const router = useRouter();
  return (
    <button
      onClick={() => {
        setToken('');
        router.push('/login');
      }}
      className="w-full rounded-lg px-3 py-2 text-left text-sm text-ink-500 transition hover:bg-ink-100 hover:text-ink-800"
    >
      退出登录
    </button>
  );
}
