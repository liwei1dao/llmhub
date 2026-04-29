'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState, type ReactNode } from 'react';
import { getToken } from '@/lib/admin-api';

export default function RequireToken({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [ready, setReady] = useState(false);
  useEffect(() => {
    if (!getToken()) {
      router.replace('/admin/login');
      return;
    }
    setReady(true);
  }, [router]);
  if (!ready) return null;
  return <>{children}</>;
}
