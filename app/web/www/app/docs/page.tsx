import Link from 'next/link';

export const metadata = {
  title: '文档',
  description: 'LLMHub 接入指南：协议、SDK、错误码、计费规则。',
};

const SECTIONS = [
  {
    title: '快速开始',
    items: [
      ['注册账户', '/docs/getting-started/sign-up'],
      ['创建 API Key', '/docs/getting-started/api-keys'],
      ['第一次调用', '/docs/getting-started/first-call'],
    ],
  },
  {
    title: '能力域',
    items: [
      ['对话 / 推理', '/docs/capabilities/chat'],
      ['向量化', '/docs/capabilities/embedding'],
      ['翻译', '/docs/capabilities/translate'],
      ['语音识别', '/docs/capabilities/asr'],
      ['语音合成', '/docs/capabilities/tts'],
    ],
  },
  {
    title: '协议',
    items: [
      ['OpenAI 兼容层', '/docs/protocols/openai'],
      ['Anthropic 兼容层', '/docs/protocols/anthropic'],
      ['错误码', '/docs/protocols/errors'],
    ],
  },
];

export default function DocsHome() {
  return (
    <main className="mx-auto max-w-5xl px-6 py-20">
      <div className="text-xs font-medium uppercase tracking-wider text-brand-600">文档</div>
      <h1 className="mt-2 text-4xl font-bold tracking-tight">从 0 到第一次调用，10 分钟</h1>
      <p className="mt-4 text-ink-500">
        所有 API 与 OpenAI / Anthropic 协议保持兼容。把 base URL 切到 LLMHub 即可，调用方代码无需修改。
      </p>

      <div className="mt-12 grid grid-cols-1 gap-6 md:grid-cols-3">
        {SECTIONS.map((s) => (
          <div key={s.title} className="rounded-2xl border border-ink-200 bg-white p-6">
            <div className="text-base font-semibold">{s.title}</div>
            <ul className="mt-4 space-y-2 text-sm">
              {s.items.map(([title, href]) => (
                <li key={href}>
                  <Link href={href} className="text-ink-700 hover:text-brand-600">
                    {title}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>

      <p className="mt-12 text-sm text-ink-500">
        文档站点持续完善中。需要的接口若未列出，请通过{' '}
        <a className="text-brand-600 underline" href="mailto:support@llmhub.io">
          support@llmhub.io
        </a>{' '}
        反馈。
      </p>
    </main>
  );
}
