const nextConfig = {
  distDir: "dist",
  transpilePackages: ["@scure/bip32"],
  serverExternalPackages: [
    "@sidiora/layerx-next",
    "@sidiora/layerx-seller-middleware",
    "@sidiora/layerx-buyer-middleware",
    "@sidiora/layerx-sdk",
  ],
};

export default nextConfig;
