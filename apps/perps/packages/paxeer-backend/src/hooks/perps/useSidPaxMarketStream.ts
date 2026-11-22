"use client";
import {
  sequenceGuard,
  isPageActive,
  scheduleActive,
  watchSocket,
  type CancelTask,
} from "@paxeer-network/paxeer-backend/core/net/realtime";

import { fetchWithPolicy } from "@paxeer-network/paxeer-backend/core/net/fetch";
import { getStreamSocketOrigin } from "@paxeer-network/paxeer-backend/core/feeds";
import {
  isVenueSymbol,
  parseVenueBook,
  parseVenueFill,
  RESYNC_DEPTH,
} from "@paxeer-network/paxeer-backend/core/net/venueStream";
import { gatewayUrls } from "@paxeer-network/paxeer-backend/core/network/gateway";
import { fundingRateFromJsonNumber } from "@paxeer-network/paxeer-backend/core/perps/fundingRate";
import { useEffect, useState } from "react";

/* ══════════════════════════════════════════════════════════════════════
   Dedicated market stream for order books, trades and stats on
   per-symbol gateway sockets.

   These servers speak a simple topic-based protocol:

     client → server :
        { event: "subscribe",   topic: "SID@orderbook" }
        { event: "subscribe",   topic: "SID@trade" }
        { event: "ping" }

     server → client :
        { topic: "SID@orderbook", data: { asks: [[p,s], ...],
                                          bids: [[p,s], ...],
                                          symbol: "SID" },
          ts?: number }
        { topic: "SID@trade",     snapshot: true,
          data:  { trades: [ { side, price, size, trade_time }, ... ] } }
        { topic: "SID@trade",
          data:  { trade: { side, price, size, trade_time } } }   // new shape
        { topic: "SID@trade",
          data:  { side: "BUY"|"SELL", price, size, trade_time } } // legacy shape
        { event: "welcome" | "subscribed" | "unsubscribed" | "pong" | "error" }

   A single WS per symbol is multiplexed across orderbook, trade and stats
   subscribers via a module-level registry,
   mirroring the pattern used in `useRealtimePrices`.
   ══════════════════════════════════════════════════════════════════════ */

const MARKET_WS_ORIGIN = getStreamSocketOrigin("");

/**
 * Symbols that have a dedicated upstream behind the market proxy. Each
 * one carries orderbook, trade tape and stats on the same WS.
 * Adding a new symbol here must be matched with a `$upstream_host` row
 * in `proxy/nginx.conf.template`.
 */
const SUPPORTED_WS_SYMBOLS = new Set([
  "BTC",
  "ETH",
  "SOL",
  "AVAX",
  "LINK",
  "HYPE",
  "XRP",
  "ASTER",
  "TRUMP",
  "BNB",
  "SID",
  "PAX",
  "TSLA",
  "NVDA",
  "NAS100",
  "SPX500",
  "GOOGL",
  "XAU",
]);

// The gateway permits the Hyperpax browser origin; no client secret is shipped.

export function buildSymbolWsUrl(symbol: string): string {
  return `${MARKET_WS_ORIGIN}/ws/${symbol.toLowerCase()}`;
}

/** True if the given display symbol is served by a dedicated market-proxy WS. */
export function hasDedicatedMarketStream(
  symbol: string | undefined | null,
): boolean {
  if (!symbol) return false;
  return SUPPORTED_WS_SYMBOLS.has(symbol.toUpperCase());
}

// ── Public types ──────────────────────────────────────────────────────
export interface SidPaxBookLevel {
  price: number;
  size: number;
}

export interface SidPaxOrderbook {
  bids: SidPaxBookLevel[]; // descending (highest price first)
  asks: SidPaxBookLevel[]; // ascending  (lowest  price first)
  ts: number; // epoch ms
}

export interface SidPaxTrade {
  side: "BUY" | "SELL";
  price: number;
  /** Number of contracts traded (new perp_trade field). */
  contracts: number;
  /** Notional dollar value of the trade. */
  notional: number;
  /** True when this trade was a liquidation event. */
  liquidation: boolean;
  trade_time: number; // epoch ms
  trade_id?: string;
  /** @deprecated Backward-compat alias — equals `notional` when present, else `contracts`. */
  size: number;
}

/**
 * Aggregated market stats pushed by the `<SYMBOL>@perp_stats` WebSocket topic.
 */
export interface SidPaxStats {
  symbol: string;
  // ── Perp-specific fields (perp_stats shape) ──
  mark_price: number;
  index_price: number;
  /** (mark − index) / index × 10 000 */
  basis_bps: number;
  /** EMA estimate of periodic funding rate, an exact decimal string; null when absent. */
  funding_rate: string | null;
  /** Last funding rate, an exact decimal string; null when absent. */
  last_funding_rate: string | null;
  /** Milliseconds until next 8-h settlement. */
  next_funding_ms: number;
  /** Rolling 5-min buy_notional / sell_notional (1.0 = balanced). */
  long_short_ratio: number;
  /** 24-h liquidation notional (USD). */
  liq_volume_24h: number;
  /** Open interest in contracts. */
  oi_contracts: number;
  /** Open interest in USD (oi_contracts × mark_price). */
  oi_usd: number;
  /** 24-h volume in contracts. */
  volume_24h_contracts: number;
  /** 24-h notional volume (USD). */
  volume_24h_usd: number;
  // ── Backward-compat aliases (used by PairHeader / MobileTradingLayout) ──
  /** @deprecated Use `volume_24h_usd`. */
  volume_24h: number;
  /** @deprecated Use `oi_usd`. */
  open_interest: number;
}

// ── Internal registry ─────────────────────────────────────────────────
interface SymbolStream {
  /** A sequence gap closed the socket; the book is fetched from REST once the socket is down. */
  bookGap?: boolean;
  url: string;
  orderbook: SidPaxOrderbook | null;
  trades: SidPaxTrade[];
  subsOrderbook: Set<(ob: SidPaxOrderbook | null) => void>;
  subsTrades: Set<(tr: SidPaxTrade[]) => void>;
  statsData: SidPaxStats | null;
  subsStats: Set<(s: SidPaxStats | null) => void>;
  ws: WebSocket | null;
  reconnectTimer: CancelTask | null;
  lingerTimer: ReturnType<typeof setTimeout> | null;
  disposed: boolean;
  reconnectDelay: number;
  pingTimer: ReturnType<typeof setInterval> | null;
}

const TRADE_CAP = 100;
const MAX_RECONNECT_DELAY = 60_000;
const INITIAL_RECONNECT_DELAY = 500;
const PING_INTERVAL = 25_000;

const streams = new Map<string, SymbolStream>();

function ensureStream(symbol: string): SymbolStream | null {
  // SSR guard — WebSocket only exists in the browser.
  if (typeof window === "undefined") return null;

  const key = symbol.toUpperCase();
  if (!SUPPORTED_WS_SYMBOLS.has(key)) return null;
  const url = buildSymbolWsUrl(key);

  let stream = streams.get(key);
  if (stream) {
    if (stream.lingerTimer) clearTimeout(stream.lingerTimer);
    stream.lingerTimer = null;
    return stream;
  }

  stream = {
    url,
    orderbook: null,
    trades: [],
    subsOrderbook: new Set(),
    subsTrades: new Set(),
    statsData: null,
    subsStats: new Set(),
    ws: null,
    reconnectTimer: null,
    lingerTimer: null,
    disposed: false,
    reconnectDelay: INITIAL_RECONNECT_DELAY,
    pingTimer: null,
  };
  streams.set(key, stream);
  const created = stream;
  queueMicrotask(() => {
    if (!created.disposed) connect(key, created);
  });
  return stream;
}

function connect(symbol: string, s: SymbolStream) {
  if (process.env.NEXT_PUBLIC_DISABLE_REALTIME_FOR_RENDER === "1") return;
  if (s.disposed) return;
  if (!isPageActive()) {
    scheduleReconnect(symbol, s);
    return;
  }
  const orderbookTopic = `${symbol}@perp_orderbook`;
  const tradeTopic = `${symbol}@perp_trade`;
  const statsTopic = `${symbol}@perp_stats`;

  let ws: WebSocket;
  try {
    ws = new WebSocket(s.url);
  } catch {
    scheduleReconnect(symbol, s);
    return;
  }
  s.ws = ws;
  watchSocket(
    ws,
    () => {
      s.reconnectDelay = INITIAL_RECONNECT_DELAY;
    },
    undefined,
    true,
  );

  ws.addEventListener("open", () => {
    try {
      ws.send(JSON.stringify({ event: "subscribe", topic: orderbookTopic }));
      ws.send(JSON.stringify({ event: "subscribe", topic: tradeTopic }));
      ws.send(JSON.stringify({ event: "subscribe", topic: statsTopic }));
    } catch {
      /* socket may have closed between open and first send */
    }
    if (!s.pingTimer) {
      s.pingTimer = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          try {
            ws.send(JSON.stringify({ event: "ping" }));
          } catch {
            /* ignore */
          }
        }
      }, PING_INTERVAL);
    }
  });

  const acceptSequence = sequenceGuard();
  ws.addEventListener("message", (ev) => {
    let msg: any;
    try {
      msg = JSON.parse(ev.data);
    } catch {
      return;
    }
    if (!acceptSequence(msg)) {
      s.bookGap = true;
      ws.close(4000, "sequence-gap");
      return;
    }

    // Order-book frame
    if (msg.topic === orderbookTopic && msg.data) {
      const book = parseVenueBook(
        msg.data,
        typeof msg.ts === "number" ? msg.ts : Date.now(),
      );
      if (!book) return;
      s.orderbook = { asks: book.asks, bids: book.bids, ts: book.ts };
      s.subsOrderbook.forEach((cb) => cb(s.orderbook));
      return;
    }

    // Trade frame — snapshot or live
    if (msg.topic === tradeTopic && msg.data) {
      if (msg.snapshot && Array.isArray(msg.data.trades)) {
        s.trades = msg.data.trades
          .slice(0, TRADE_CAP)
          .map(parseSidPaxTrade)
          .filter(
            (trade: SidPaxTrade | null): trade is SidPaxTrade => trade !== null,
          );
        s.subsTrades.forEach((cb) => cb(s.trades));
      } else if (msg.data.trade) {
        // New shape: `data.trade` is the full trade object.
        const t = parseSidPaxTrade(msg.data.trade);
        if (!t) return;
        s.trades = [t, ...s.trades].slice(0, TRADE_CAP);
        s.subsTrades.forEach((cb) => cb(s.trades));
      } else if (msg.data.side && msg.data.price != null) {
        // Legacy shape: `data` itself is the trade object.
        const t = parseSidPaxTrade(msg.data);
        if (!t) return;
        s.trades = [t, ...s.trades].slice(0, TRADE_CAP);
        s.subsTrades.forEach((cb) => cb(s.trades));
      }
      return;
    }

    // Stats frame — perp_stats shape (full perp metrics snapshot).
    if (msg.topic === statsTopic && msg.data) {
      const stats = parseSidPaxStats(msg.data, symbol);
      if (!stats) return;
      s.statsData = stats;
      s.subsStats.forEach((cb) => cb(s.statsData));
      return;
    }

    // Control frames — welcome / subscribed / unsubscribed / pong / error
    // are informational only.
  });

  const onDisconnect = () => {
    if (s.disposed) return;
    s.orderbook = null;
    s.trades = [];
    s.statsData = null;
    s.subsOrderbook.forEach((cb) => cb(null));
    s.subsTrades.forEach((cb) => cb([]));
    s.subsStats.forEach((cb) => cb(null));
    if (s.pingTimer) {
      clearInterval(s.pingTimer);
      s.pingTimer = null;
    }
    if (s.ws === ws) s.ws = null;
    if (s.bookGap) {
      s.bookGap = false;
      void resyncBook(symbol, s);
    }
    scheduleReconnect(symbol, s);
  };
  ws.addEventListener("close", onDisconnect);
  ws.addEventListener("error", () => {
    // `close` will also fire; leave reconnection to it.
  });
}

/** The REST book carries no seq: the subscribe snapshot after the reconnect replaces it. */
async function resyncBook(symbol: string, s: SymbolStream) {
  if (!isVenueSymbol(symbol)) return;
  const url = `${gatewayUrls().restApiUrl}/orderbook/${symbol}?depth=${RESYNC_DEPTH}`;
  try {
    const res = await fetchWithPolicy(url);
    const json = (await res.json()) as { success?: unknown; data?: unknown };
    if (json.success !== true || s.disposed || s.orderbook !== null) return;
    const book = parseVenueBook(json.data, Date.now());
    if (!book) return;
    s.orderbook = { asks: book.asks, bids: book.bids, ts: book.ts };
    s.subsOrderbook.forEach((cb) => cb(s.orderbook));
  } catch {
    /* the subscribe snapshot restores the book */
  }
}

function scheduleReconnect(symbol: string, s: SymbolStream) {
  if (s.reconnectTimer || s.disposed) return;
  const delay = s.reconnectDelay;
  s.reconnectTimer = scheduleActive(
    () => {
      s.reconnectTimer = null;
      s.reconnectDelay = Math.min(MAX_RECONNECT_DELAY, s.reconnectDelay * 2);
      connect(symbol, s);
    },
    delay,
    s.url,
  );
}

function releaseIfIdle(symbol: string, s: SymbolStream) {
  const listeners = () =>
    s.subsOrderbook.size + s.subsTrades.size + s.subsStats.size;
  if (listeners() || s.lingerTimer) return;
  s.lingerTimer = setTimeout(() => {
    s.lingerTimer = null;
    if (listeners()) return;
    s.disposed = true;
    s.reconnectTimer?.();
    if (s.pingTimer) clearInterval(s.pingTimer);
    s.ws?.close(1000, "no-subscribers");
    s.ws = null;
    streams.delete(symbol.toUpperCase());
  }, 15000);
}

/** Decode the gateway's perp_trade payload without inventing a side or timestamp. */
export function parseSidPaxTrade(raw: unknown): SidPaxTrade | null {
  const trade = parseVenueFill(raw);
  if (!trade) return null;
  return {
    side: trade.side,
    price: trade.price,
    contracts: trade.contracts,
    notional: trade.notional,
    liquidation: trade.liquidation,
    size: trade.notional,
    trade_id: trade.id ?? undefined,
    trade_time: trade.trade_time,
  };
}

/** The gateway shares volume_24h and open_interest between stats and perp_stats. */
export function parseSidPaxStats(
  raw: unknown,
  symbol: string,
): SidPaxStats | null {
  if (!raw || typeof raw !== "object") return null;
  const d = raw as Record<string, unknown>;
  if (
    d.symbol !== symbol ||
    typeof d.volume_24h !== "number" ||
    !Number.isFinite(d.volume_24h) ||
    typeof d.open_interest !== "number" ||
    !Number.isFinite(d.open_interest)
  )
    return null;
  const volumeUsd = d.volume_24h;
  const oiUsd = d.open_interest;
  return {
    symbol: String(d.symbol ?? symbol),
    mark_price: Number(d.mark_price) || 0,
    index_price: Number(d.index_price) || 0,
    basis_bps: Number(d.basis_bps) || 0,
    // perp_stats sends both funding rates as JSON numbers; fundingRateFromJsonNumber is the one conversion.
    funding_rate: fundingRateFromJsonNumber(d.funding_rate),
    last_funding_rate: fundingRateFromJsonNumber(d.last_funding_rate),
    next_funding_ms: Number(d.next_funding_ms) || 0,
    long_short_ratio: Number(d.long_short_ratio) || 1,
    liq_volume_24h: Number(d.liq_volume_24h) || 0,
    oi_contracts: Number(d.oi_contracts) || 0,
    oi_usd: oiUsd,
    volume_24h_contracts: Number(d.volume_24h_contracts) || 0,
    volume_24h_usd: volumeUsd,
    // backward-compat aliases
    volume_24h: volumeUsd,
    open_interest: oiUsd,
  };
}

/** Order-book listener for a dedicated stream: the cached book, each frame, and null while the socket is down. */
export function subscribeOrderbook(
  symbol: string,
  cb: (book: SidPaxOrderbook | null) => void,
): () => void {
  const s = ensureStream(symbol);
  if (!s) return () => {};
  s.subsOrderbook.add(cb);
  if (s.orderbook) cb(s.orderbook);
  return () => {
    s.subsOrderbook.delete(cb);
    releaseIfIdle(symbol, s);
  };
}

/** Native workspace order-book stream adapter. */
export function subscribeNativeOrderbook(
  symbol: string,
  cb: (book: SidPaxOrderbook | null) => void,
): () => void {
  return subscribeOrderbook(symbol, cb);
}

/** Native workspace trade stream adapter. */
export function subscribeNativeTrades(
  symbol: string,
  cb: (trades: SidPaxTrade[]) => void,
): () => void {
  const s = ensureStream(symbol);
  if (!s) return () => {};
  s.subsTrades.add(cb);
  cb(s.trades);
  return () => {
    s.subsTrades.delete(cb);
    releaseIfIdle(symbol, s);
  };
}

// ── React hooks ───────────────────────────────────────────────────────
/**
 * Subscribe to the dedicated order-book feed for SID or PAX.
 * Returns `null` until the first frame arrives, or if the symbol has no
 * dedicated stream configured.
 */
export function useSidPaxOrderbook(
  symbol: string | undefined,
): SidPaxOrderbook | null {
  const [book, setBook] = useState<SidPaxOrderbook | null>(null);

  useEffect(() => {
    if (!symbol) {
      setBook(null);
      return;
    }
    const s = ensureStream(symbol);
    if (!s) {
      setBook(null);
      return;
    }
    // Sync any cached value captured before the first render.
    setBook(s.orderbook);
    return subscribeOrderbook(symbol, setBook);
  }, [symbol]);

  return book;
}

/**
 * Subscribe to the dedicated trade tape for SID or PAX.
 * Always returns newest-first, capped at `TRADE_CAP` rows.
 */
export function useSidPaxTrades(symbol: string | undefined): SidPaxTrade[] {
  const [trades, setTrades] = useState<SidPaxTrade[]>([]);

  useEffect(() => {
    if (!symbol) {
      setTrades([]);
      return;
    }
    const s = ensureStream(symbol);
    if (!s) {
      setTrades([]);
      return;
    }
    setTrades(s.trades);
    s.subsTrades.add(setTrades);
    return () => {
      s.subsTrades.delete(setTrades);
      releaseIfIdle(symbol, s);
    };
  }, [symbol]);

  return trades;
}

/**
 * Subscribe to the `@stats` feed — delivers 24h volume, open interest,
 * and estimated + last funding rates. Returns `null` until the first
 * frame arrives or if the symbol has no dedicated stream.
 */
export function useSidPaxStats(symbol: string | undefined): SidPaxStats | null {
  const [stats, setStats] = useState<SidPaxStats | null>(null);

  useEffect(() => {
    if (!symbol) {
      setStats(null);
      return;
    }
    const s = ensureStream(symbol);
    if (!s) {
      setStats(null);
      return;
    }
    setStats(s.statsData);
    s.subsStats.add(setStats);
    return () => {
      s.subsStats.delete(setStats);
      releaseIfIdle(symbol, s);
    };
  }, [symbol]);

  return stats;
}

/** Map TradingView resolutions to canonical candle intervals for series keys. */
export function tvResolutionToKlineInterval(
  resolution: string | undefined | null,
): string | null {
  if (!resolution) return null;
  const r = String(resolution).toUpperCase();
  switch (r) {
    case "1":
      return "1m";
    case "2":
      return "2m";
    case "3":
      return "3m";
    case "5":
      return "5m";
    case "15":
      return "15m";
    case "30":
      return "30m";
    case "60":
      return "1h";
    case "240":
      return "4h";
    case "480":
      return "8h";
    case "D":
    case "1D":
      return "1D";
    case "W":
    case "1W":
      return "1W";
    default:
      return null;
  }
}
