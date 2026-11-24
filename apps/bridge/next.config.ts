import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'standalone',
  outputFileTracingRoot: import.meta.dirname,
  cacheComponents: true,
  partialPrefetching: true,
  turbopack: {
    root: import.meta.dirname,
    rules: {
      '*.css': {
        loaders: [ '@tailwindcss/turbopack' ],
        as: '*.css',
      },
    },
  },
};

export default nextConfig;
