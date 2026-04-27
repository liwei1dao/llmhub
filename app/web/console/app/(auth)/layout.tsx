import type { ReactNode } from 'react';

export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-screen place-items-center bg-gradient-to-br from-white via-slate-50 to-blue-50 px-6">
      <div className="w-full max-w-md">
        <div className="mb-8 flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-gradient-to-br from-brand-600 to-fuchsia-600 text-base font-semibold text-white">
            L
          </span>
          <span className="text-xl font-semibold tracking-tight">LLMHub</span>
        </div>
        {children}
      </div>
    </div>
  );
}
