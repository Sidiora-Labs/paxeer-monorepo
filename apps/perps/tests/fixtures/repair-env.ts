// Evaluate before importing any production module, whose configuration is read
// at module load. All networked proofs bind their own loopback fixture servers.
process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "http://127.0.0.1";
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = "http://127.0.0.1";
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = "http://127.0.0.1/api";
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = "ws://127.0.0.1";
