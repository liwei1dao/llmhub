import type { NextConfig } from 'next';

/** User console — SPA-like, behind login. SEO not required. */
const config: NextConfig = {
  poweredByHeader: false,
  reactStrictMode: true,
};

export default config;
