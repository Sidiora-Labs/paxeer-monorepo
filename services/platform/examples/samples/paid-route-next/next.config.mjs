const nextConfig = {
  distDir: "dist",
  // BIP32's base encoding dependency can expose TypeScript in a private SDK install.
  transpilePackages: ["@scure/bip32", "@scure/base"],
  serverExternalPackages: [
    "@sidiora/layerx-next",
    "@sidiora/layerx-seller-middleware",
    "@sidiora/layerx-buyer-middleware",
    "@sidiora/layerx-sdk",
  ],
};

export default nextConfig;
