import Link from 'next/link';
import type { ReactNode } from 'react';
import LogoutButton from './_components/logout-button';
import RequireToken from './_components/require-token';

const NAV = [
  { href: '/admin/dashboard', label: '总览' },
  { href: '/admin/pool', label: '账号池' },
  { href: '/admin/users', label: '用户' },
  { href: '/admin/providers', label: '厂商' },
  { href: '/admin/pricing', label: '定价' },
  { href: '/admin/recharges', label: '充值确认' },
  { href: '/admin/recon', label: '对账' },
];

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <RequireToken>
      <div className="flex min-h-screen">
        <aside className="hidden w-60 shrink-0 border-r border-ink-700 bg-ink-800 px-4 py-6 md:block">
          <Link href="/admin/dashboard" className="flex items-center gap-2 px-2 pb-6">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-rose-500 to-orange-500 text-sm font-semibold text-white">
              A
            </span>
            <span className="font-semibold tracking-tight text-white">Admin</span>
          </Link>
          <nav className="space-y-1">
            {NAV.map((n) => (
              <Link
                key={n.href}
                href={n.href}
                className="block rounded-lg px-3 py-2 text-sm text-ink-200 transition hover:bg-ink-700"
              >
                {n.label}
              </Link>
            ))}
          </nav>
          <div className="mt-8 border-t border-ink-700 pt-4">
            <LogoutButton />
          </div>
        </aside>
        <main className="flex-1 px-6 py-8 md:px-10">{children}</main>
      </div>
    </RequireToken>
  );
}
