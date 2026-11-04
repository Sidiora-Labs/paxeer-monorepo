import { fileURLToPath } from "node:url";

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  assetPrefix: "/human-ui",
  reactStrictMode: true,
  // src/api/sdk.ts imports the agent SDK's TypeScript sources, and those modules
  // name each other with the .js specifier their own NodeNext compiler requires.
  // The bundler has to try the TypeScript file behind such a specifier, which is
  // why the build runs on webpack: Turbopack exposes no extension alias.
  experimental: {
    extensionAlias: {
      ".js": [".js", ".ts", ".tsx"],
    },
  },
  webpack(config) {
    config.module.rules.push({
      include: fileURLToPath(new URL("../../sdk/wallet/src", import.meta.url)),
      resolve: {
        modules: [
          fileURLToPath(new URL("./node_modules", import.meta.url)),
          ...config.resolve.modules,
        ],
      },
    });
    return config;
  },
  async rewrites() {
    return [
      {
        source: "/human/v1/:path*",
        destination: "/api/human/v1/:path*",
      },
      {
        source: "/v1/:path*",
        destination: "/api/human/v1/:path*",
      },
      {
        // This is Human readiness; ramp readiness has a separate service URL.
        source: "/readyz",
        destination: "/api/human/readyz",
      },
    ];
  },
  async headers() {
    return [
      {
        source: "/readyz",
        headers: [{ key: "Cache-Control", value: "no-store" }],
      },
      {
        source: "/explorer",
        headers: [
          {
            key: "Cache-Control",
            value: "public, s-maxage=60, stale-while-revalidate=300",
          },
        ],
      },
      {
        source: "/explorer/:path*",
        headers: [
          {
            key: "Cache-Control",
            value: "public, s-maxage=60, stale-while-revalidate=300",
          },
        ],
      },
      {
        source: "/api/performance/vitals",
        headers: [{ key: "Cache-Control", value: "no-store" }],
      },
    ];
  },
};

export default nextConfig;
