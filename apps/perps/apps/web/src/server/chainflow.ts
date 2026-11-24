import ChainflowDataGateway from "@sidioralabs/chainflow-data-gateway";

const FEED_ATTEMPTS = 3;
const FEED_ATTEMPT_TIMEOUT = 3_800;
const FEED_RETRY_PAUSE = 200;
const RETRYABLE_STATUSES = new Set([502, 503, 504]);

async function fetchFeedAttempt(input: RequestInfo | URL, init?: RequestInit) {
  init?.signal?.throwIfAborted();
  const controller = new AbortController();
  let abortAttempt!: () => void;
  const aborted = new Promise<never>((_, reject) => {
    abortAttempt = () => {
      controller.abort();
      reject(controller.signal.reason);
    };
  });
  init?.signal?.addEventListener("abort", abortAttempt, { once: true });
  const timeout = setTimeout(abortAttempt, FEED_ATTEMPT_TIMEOUT);
  try {
    return await Promise.race([
      globalThis.fetch(input, {
        ...init,
        signal: controller.signal,
        redirect: "error",
      }),
      aborted,
    ]);
  } finally {
    clearTimeout(timeout);
    init?.signal?.removeEventListener("abort", abortAttempt);
  }
}

async function fetchFeed(input: RequestInfo | URL, init?: RequestInit) {
  for (let attempt = 1; ; attempt++) {
    let response: Response;
    try {
      response = await fetchFeedAttempt(input, init);
    } catch (error) {
      if (init?.signal?.aborted || attempt === FEED_ATTEMPTS) throw error;
      await new Promise<void>((resolve) =>
        setTimeout(resolve, FEED_RETRY_PAUSE),
      );
      continue;
    }
    if (!RETRYABLE_STATUSES.has(response.status) || attempt === FEED_ATTEMPTS)
      return response;
    await response.body?.cancel();
    await new Promise<void>((resolve) => setTimeout(resolve, FEED_RETRY_PAUSE));
  }
}

/** Each credential has one purpose; the generated client's mint header overrides its REST header. */
export function chainflowClient(purpose: "rest" | "mint" = "rest") {
  const restAPIKey = process.env.PRICE_FEED_REST_API_KEY;
  const wsMintKey =
    purpose === "mint" ? process.env.PRICE_FEED_WS_API_KEY : null;
  if (!restAPIKey || (purpose === "mint" && !wsMintKey))
    throw new Error("feed_not_configured");
  return new ChainflowDataGateway({
    restAPIKey,
    wsMintKey,
    wsToken: null,
    baseURL:
      process.env.PRICE_FEED_ORIGIN || "https://data.feed.chainflowtrading.com",
    timeout: 12_000,
    maxRetries: 0,
    logLevel: "off",
    fetch: fetchFeed,
  });
}
