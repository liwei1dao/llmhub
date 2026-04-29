import type { Metadata } from 'next';
import type { ReactNode } from 'react';

// Admin 段统一暗色背景。root layout 不染色，营销/console 用浅色，
// 这里通过最外层 div 切到 ink-900 + ink-200 文字。
export const metadata: Metadata = {
  title: 'LLMHub Admin',
  description: 'LLMHub 运营后台',
  robots: { index: false, follow: false },
};

export default function AdminSegmentLayout({ children }: { children: ReactNode }) {
  return <div className="min-h-screen bg-ink-900 text-ink-200">{children}</div>;
}
