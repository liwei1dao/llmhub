import type { MetadataRoute } from 'next';

// Extend as routes are added. Sitemap is served at /sitemap.xml.
export default function sitemap(): MetadataRoute.Sitemap {
  const base = 'https://llmhub.io';
  return [
    { url: `${base}/`, lastModified: new Date(), changeFrequency: 'weekly', priority: 1 },
    { url: `${base}/pricing`, changeFrequency: 'weekly', priority: 0.8 },
    { url: `${base}/capabilities`, changeFrequency: 'weekly', priority: 0.8 },
    { url: `${base}/docs`, changeFrequency: 'weekly', priority: 0.6 },
  ];
}
