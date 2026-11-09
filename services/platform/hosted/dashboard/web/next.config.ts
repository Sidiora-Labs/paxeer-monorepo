import path from "node:path";
import type { NextConfig } from "next";

const repositoryRoot = path.resolve(import.meta.dirname, "../../../../..");
const dashboardApiOrigin =
  process.env.DASHBOARD_API_ORIGIN ||
  (process.env.NODE_ENV !== "production" ? "http://127.0.0.1:9445" : undefined);

if (!dashboardApiOrigin) {
  throw new Error("DASHBOARD_API_ORIGIN must be set in production");
}

const config: NextConfig = {
  output: "standalone",
  outputFileTracingRoot: repositoryRoot,
  poweredByHeader: false,
  reactStrictMode: true,
  turbopack: {
    root: repositoryRoot,
  },
  async rewrites() {
    return [
      {
        source: "/v1/dashboard/:path*",
        destination: `${dashboardApiOrigin.replace(/\/+$/, "")}/v1/dashboard/:path*`,
      },
    ];
  },
};

export default config;
