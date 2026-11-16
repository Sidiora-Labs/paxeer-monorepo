/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  experimental: {
    // Larger payload caps if a partner app passes complex tx data.
    serverActions: { bodySizeLimit: '2mb' },
  },
};

export default nextConfig;
