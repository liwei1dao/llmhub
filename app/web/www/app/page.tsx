import Link from 'next/link';

const PROVIDERS = [
  { id: 'volc', name: '火山方舟', tag: 'Volcengine Ark', tone: 'from-orange-500 to-red-500' },
  { id: 'deepseek', name: 'DeepSeek', tag: '深度求索', tone: 'from-indigo-500 to-blue-500' },
  { id: 'anthropic', name: 'Anthropic', tag: 'Claude 系列', tone: 'from-amber-500 to-orange-600' },
  { id: 'dashscope', name: '阿里百炼', tag: 'DashScope', tone: 'from-sky-500 to-cyan-500' },
  { id: 'deepl', name: 'DeepL', tag: '翻译', tone: 'from-emerald-500 to-teal-500' },
  { id: 'elevenlabs', name: 'ElevenLabs', tag: 'TTS', tone: 'from-purple-500 to-fuchsia-500' },
  { id: 'deepgram', name: 'Deepgram', tag: 'ASR', tone: 'from-pink-500 to-rose-500' },
];

const CAPABILITIES = [
  { id: 'chat', name: '对话 / 推理', desc: 'OpenAI / Anthropic 协议兼容，SSE 流式转发，token 实时计费。' },
  { id: 'embedding', name: '向量化', desc: '统一 dimension / batch 接口，支持火山、DeepSeek、阿里。' },
  { id: 'translate', name: '翻译', desc: '语种自动识别，多家厂商灰度路由，跨厂商质量比对。' },
  { id: 'asr', name: '语音识别', desc: 'WebSocket / 长音频，按音频秒数计费。' },
  { id: 'tts', name: '语音合成', desc: 'SSML、克隆音色，按字符数 / 音频秒数计费。' },
  { id: 'multimodal', name: '多模态', desc: '图像理解 / 视觉对话，规划中。' },
];

export default function HomePage() {
  return (
    <main className="min-h-screen">
      <Header />
      <Hero />
      <Providers />
      <Capabilities />
      <Pricing />
      <CTA />
      <Footer />
    </main>
  );
}

function Header() {
  return (
    <header className="sticky top-0 z-30 border-b border-ink-200/60 bg-white/70 backdrop-blur">
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
        <Link href="/" className="flex items-center gap-2">
          <span className="grid h-8 w-8 place-items-center rounded-lg bg-gradient-to-br from-brand-600 to-fuchsia-600 text-sm font-semibold text-white">
            L
          </span>
          <span className="text-lg font-semibold tracking-tight">LLMHub</span>
        </Link>
        <nav className="hidden items-center gap-7 text-sm text-ink-700 md:flex">
          <a href="#providers">厂商</a>
          <a href="#capabilities">能力</a>
          <a href="#pricing">定价</a>
          <Link href="/docs">文档</Link>
        </nav>
        <div className="flex items-center gap-3">
          <a href="http://localhost:3001" className="hidden text-sm text-ink-700 md:inline">
            登录
          </a>
          <a
            href="http://localhost:3001/register"
            className="rounded-lg bg-ink-900 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-ink-800"
          >
            免费注册
          </a>
        </div>
      </div>
    </header>
  );
}

function Hero() {
  return (
    <section className="glow relative overflow-hidden">
      <div className="grid-bg absolute inset-0 opacity-60" />
      <div className="relative mx-auto max-w-6xl px-6 py-24 md:py-32">
        <div className="mx-auto max-w-3xl text-center">
          <span className="inline-flex items-center gap-2 rounded-full border border-brand-100 bg-white/70 px-3 py-1 text-xs font-medium text-brand-700">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            7 家厂商已接入 · 协议层一次对齐
          </span>
          <h1 className="mt-6 text-5xl font-bold leading-tight tracking-tight md:text-6xl">
            一次接入<br />
            <span className="bg-gradient-to-r from-brand-600 via-fuchsia-600 to-pink-600 bg-clip-text text-transparent">
              全栈 AI 能力
            </span>
            可用
          </h1>
          <p className="mt-6 text-lg text-ink-500 md:text-xl">
            对话 · 语音 · 翻译 · 多模态 —— 用最简单的 API 调遍主流大模型厂商。
            <br className="hidden md:block" />
            按量计费，无套餐门槛，毫秒级故障切换。
          </p>
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <a
              href="http://localhost:3001/register"
              className="rounded-xl bg-ink-900 px-6 py-3 text-sm font-medium text-white shadow-sm hover:bg-ink-800"
            >
              立即开始 →
            </a>
            <Link
              href="/docs"
              className="rounded-xl border border-ink-200 bg-white px-6 py-3 text-sm font-medium text-ink-800 hover:bg-ink-50"
            >
              阅读文档
            </Link>
          </div>
          <CodeSample />
        </div>
      </div>
    </section>
  );
}

function CodeSample() {
  return (
    <pre className="mono mt-12 overflow-x-auto rounded-2xl border border-ink-200 bg-ink-900 p-6 text-left text-sm leading-relaxed text-ink-200 shadow-xl">
      <code>{`curl https://api.llmhub.io/v1/chat/completions \\
  -H "Authorization: Bearer $LLMHUB_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "deepseek-chat",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'`}</code>
    </pre>
  );
}

function Providers() {
  return (
    <section id="providers" className="border-t border-ink-200/60 bg-white py-20">
      <div className="mx-auto max-w-6xl px-6">
        <SectionHead
          eyebrow="覆盖厂商"
          title="主流大模型厂商，一站式调度"
          desc="出问题自动切换，账单按毛利对账，避免单点。"
        />
        <div className="mt-12 grid grid-cols-2 gap-4 md:grid-cols-4 lg:grid-cols-7">
          {PROVIDERS.map((p) => (
            <div
              key={p.id}
              className="group rounded-xl border border-ink-200 bg-white p-4 text-center transition hover:-translate-y-0.5 hover:shadow-md"
            >
              <div
                className={`mx-auto mb-3 grid h-10 w-10 place-items-center rounded-lg bg-gradient-to-br ${p.tone} text-sm font-semibold text-white`}
              >
                {p.name.slice(0, 1)}
              </div>
              <div className="text-sm font-medium">{p.name}</div>
              <div className="mt-0.5 text-xs text-ink-500">{p.tag}</div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function Capabilities() {
  return (
    <section id="capabilities" className="bg-ink-900/[0.02] py-20">
      <div className="mx-auto max-w-6xl px-6">
        <SectionHead
          eyebrow="能力域"
          title="按能力域抽象，无需关心底层差异"
          desc="LLMHub 在协议层做了厂商对齐，业务方只对接一套 SDK。"
        />
        <div className="mt-12 grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          {CAPABILITIES.map((c) => (
            <div key={c.id} className="rounded-2xl border border-ink-200 bg-white p-6">
              <div className="text-base font-semibold">{c.name}</div>
              <p className="mt-2 text-sm text-ink-500">{c.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function Pricing() {
  return (
    <section id="pricing" className="border-t border-ink-200/60 bg-white py-20">
      <div className="mx-auto max-w-6xl px-6">
        <SectionHead eyebrow="计费" title="按量计费，无套餐门槛" desc="账单透明、可对账、可导出。" />
        <div className="mt-12 grid grid-cols-1 gap-6 md:grid-cols-3">
          <Plan
            tier="入门"
            price="¥0 起充"
            features={['共享池调度', '20 QPS', '邮件支持', '所有公开模型']}
          />
          <Plan
            tier="增长"
            price="¥1,000/月起"
            highlight
            features={['专属调度优先级', '200 QPS', '工单 + 群', 'SLA 99.9%']}
          />
          <Plan
            tier="企业"
            price="商务定制"
            features={['专属账号池', '不限 QPS', '独立 VPC / 私有部署', '24×7 支持 + SLA 99.99%']}
          />
        </div>
      </div>
    </section>
  );
}

function Plan({
  tier,
  price,
  features,
  highlight,
}: {
  tier: string;
  price: string;
  features: string[];
  highlight?: boolean;
}) {
  return (
    <div
      className={`rounded-2xl border bg-white p-6 ${
        highlight ? 'border-brand-500 shadow-lg shadow-brand-500/10' : 'border-ink-200'
      }`}
    >
      <div className="text-sm font-medium text-ink-500">{tier}</div>
      <div className="mt-2 text-3xl font-bold tracking-tight">{price}</div>
      <ul className="mt-6 space-y-2 text-sm text-ink-700">
        {features.map((f) => (
          <li key={f} className="flex items-start gap-2">
            <span className="mt-0.5 text-brand-600">✓</span>
            <span>{f}</span>
          </li>
        ))}
      </ul>
      <a
        href="http://localhost:3001/register"
        className={`mt-8 block rounded-xl px-4 py-2.5 text-center text-sm font-medium ${
          highlight
            ? 'bg-ink-900 text-white hover:bg-ink-800'
            : 'border border-ink-200 text-ink-800 hover:bg-ink-50'
        }`}
      >
        立即开通
      </a>
    </div>
  );
}

function CTA() {
  return (
    <section className="bg-ink-900 py-20 text-white">
      <div className="mx-auto max-w-4xl px-6 text-center">
        <h2 className="text-3xl font-bold tracking-tight md:text-4xl">几分钟接通你的第一次调用</h2>
        <p className="mt-4 text-ink-200">注册即送测试额度，文档与样例代码开箱即用。</p>
        <a
          href="http://localhost:3001/register"
          className="mt-8 inline-block rounded-xl bg-white px-6 py-3 text-sm font-semibold text-ink-900 hover:bg-ink-200"
        >
          免费创建 API Key
        </a>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="border-t border-ink-200 bg-white py-10 text-sm text-ink-500">
      <div className="mx-auto flex max-w-6xl flex-col items-start justify-between gap-4 px-6 md:flex-row md:items-center">
        <div>© {new Date().getFullYear()} LLMHub · AI 能力聚合与分销平台</div>
        <div className="flex gap-5">
          <Link href="/docs">文档</Link>
          <Link href="/legal/terms">服务条款</Link>
          <Link href="/legal/privacy">隐私</Link>
        </div>
      </div>
    </footer>
  );
}

function SectionHead({ eyebrow, title, desc }: { eyebrow: string; title: string; desc: string }) {
  return (
    <div className="mx-auto max-w-3xl text-center">
      <div className="text-xs font-medium uppercase tracking-wider text-brand-600">{eyebrow}</div>
      <h2 className="mt-2 text-3xl font-bold tracking-tight md:text-4xl">{title}</h2>
      <p className="mt-3 text-base text-ink-500">{desc}</p>
    </div>
  );
}
