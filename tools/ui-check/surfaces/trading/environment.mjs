export const environment = {
  ...process.env,
  NEXT_PUBLIC_PERPS_GATEWAY_URL: "http://localhost:4173",
  NEXT_PUBLIC_PAXEER_RPC_URLS: "http://localhost:4173/rpc",
  NEXT_PUBLIC_DIRECT_PRICE_FEED_ORIGINS: "https://app.hyperpax.xyz",
  NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY: "",
  NEXT_PUBLIC_PAXEER_WALLET_API: "",
  NEXT_PUBLIC_PAXSCAN_API_URL: "http://localhost:4173/paxscan",
  NEXT_PUBLIC_PAXSCAN_WS_URL: "ws://localhost:4173/paxscan",
  NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER: "1",
  PAXEER_NETWORK_BUILD_DIR: ".next-production",
  NEXT_TELEMETRY_DISABLED: "1",
};
