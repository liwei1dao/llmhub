import Link from 'next/link';
import type { ReactNode } from 'react';
import LogoutButton from './_components/logout-button';
import RequireToken from './_components/require-token';

const NAV: { group: string; items: { href: string; label: string }[] }[] = [
  {
    group: '',
    items: [{ href: '/dashboard', label: '📊 仪表盘' }],
  },
  {
    group: '资源',
    items: [
      { href: '/accounts', label: '🪪 账号管理' },
      { href: '/credentials', label: '🔑 凭据管理' },
    ],
  },
  {
    group: '运营',
    items: [
      { href: '/users', label: '👥 用户' },
      { href: '/audit', label: '🔍 审计日志' },
    ],
  },
];

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <RequireToken>
      <div className="flex min-h-screen">
        <aside className="hidden w-60 shrink-0 border-r border-ink-200 bg-white px-4 py-6 md:block">
          <Link href="/dashboard" className="flex items-center gap-2 px-2 pb-6">
            <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-rose-500 to-orange-500 text-sm font-semibold text-white">
              A
            </span>
            <span className="font-semibold tracking-tight text-ink-900">Admin</span>
          </Link>
          <nav className="space-y-4">
            {NAV.map((g, i) => (
              <div key={i}>
                {g.group ? (
                  <div className="px-3 mb-1.5 text-[11px] uppercase tracking-wider text-ink-500">
                    {g.group}
                  </div>
                ) : null}
                <div className="space-y-0.5">
                  {g.items.map((n) => (
                    <Link
                      key={n.href}
                      href={n.href}
                      className="block rounded-lg px-3 py-2 text-sm text-ink-800 transition hover:bg-ink-100"
                    >
                      {n.label}
                    </Link>
                  ))}
                </div>
              </div>
            ))}
          </nav>
          <div className="mt-8 border-t border-ink-200 pt-4">
            <LogoutButton />
          </div>
        </aside>
        <main className="flex-1 px-6 py-8 md:px-10">{children}</main>
      </div>
    </RequireToken>
  );
}
