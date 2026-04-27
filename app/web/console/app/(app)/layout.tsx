import Link from 'next/link';
import type { ReactNode } from 'react';
import LogoutButton from './_components/logout-button';

const NAV = [
  { href: '/dashboard', label: '总览' },
  { href: '/api-keys', label: 'API Key' },
  { href: '/wallet', label: '钱包' },
  { href: '/usage', label: '用量' },
];

export default function AppLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-60 shrink-0 border-r border-ink-200 bg-white px-4 py-6 md:block">
        <Link href="/dashboard" className="flex items-center gap-2 px-2 pb-6">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-brand-600 to-fuchsia-600 text-sm font-semibold text-white">
            L
          </span>
          <span className="font-semibold tracking-tight">Console</span>
        </Link>
        <nav className="space-y-1">
          {NAV.map((n) => (
            <Link
              key={n.href}
              href={n.href}
              className="block rounded-lg px-3 py-2 text-sm text-ink-700 transition hover:bg-ink-200/40"
            >
              {n.label}
            </Link>
          ))}
        </nav>
        <div className="mt-8 border-t border-ink-200 pt-4">
          <LogoutButton />
        </div>
      </aside>
      <main className="flex-1 px-6 py-8 md:px-10">{children}</main>
    </div>
  );
}
