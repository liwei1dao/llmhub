import type { NextConfig } from 'next';

/**
 * Marketing site (llmhub.io).
 *
 * SEO-first: prefer SSG/ISR whenever possible. The App Router handles this
 * automatically for pages without dynamic server data.
 */
const config: NextConfig = {
  poweredByHeader: false,
  reactStrictMode: true,
  experimental: {
    // Reserved for future opt-ins (PPR, cache components, etc.).
  },
};

export default config;
