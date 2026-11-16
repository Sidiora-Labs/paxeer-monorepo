import path from 'node:path';
import { fileURLToPath } from 'node:url';
import type { NextConfig } from 'next';

const appDir = path.dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  output: 'standalone',
  outputFileTracingRoot: path.resolve(appDir, '../..'),
  poweredByHeader: false,
  trailingSlash: false,
  skipTrailingSlashRedirect: true,
};

export default nextConfig;
