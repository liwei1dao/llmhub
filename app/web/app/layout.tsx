import type { Metadata } from 'next';
import type { ReactNode } from 'react';
import './globals.css';

export const metadata: Metadata = {
  metadataBase: new URL('https://llmhub.io'),
  title: {
    default: 'LLMHub · AI 能力聚合与分销平台',
    template: '%s · LLMHub',
  },
  description:
    '一次接入，全栈 AI 能力可用 —— 对话、语音、翻译、多模态。兼容 OpenAI / Anthropic 协议，按量计费，无套餐门槛。',
  keywords: ['LLM', 'AI', '大模型', '聚合', 'OpenAI', 'Anthropic', 'DeepSeek', '火山方舟'],
  openGraph: {
    type: 'website',
    locale: 'zh_CN',
    url: 'https://llmhub.io',
    siteName: 'LLMHub',
  },
  twitter: { card: 'summary_large_image' },
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
