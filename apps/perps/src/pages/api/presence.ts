import type { NextApiRequest, NextApiResponse } from "next";
import { randomBytes } from "node:crypto";
import { isIP } from "node:net";

const TTL_MS = 15_000;
const HISTORY_SLOTS = 12;
const HISTORY_INTERVAL_MS = 60_000;
const MAX_SESSIONS = 50_000;
const IDENTITY_TTL_MS = 5 * 60_000;
const REGISTRATION_WINDOW_MS = 60_000;
const MAX_CLIENT_IDENTITIES = 5;
const MAX_CLIENTS = 5_000;
const SESSION_COOKIE = "pax-presence-id";

interface Session {
  client: string;
  market: string;
  expiresAt: number;
  identityExpiresAt: number;
  joinedAt: number;
}

interface ClientBudget {
  identities: Set<string>;
  registrations: number;
  windowStart: number;
}

interface HistorySlot {
  at: number;
  total: number;
  byMarket: Record<string, number>;
}

interface PresenceSnapshot {
  total: number;
  byMarket: Record<string, number>;
  top: Array<{ market: string; count: number; delta5m: number }>;
  rising: Array<{ market: string; count: number; deltaPct: number }>;
  history: Array<{ at: number; total: number }>;
  lastUpdate: string;
  window: { ttlSeconds: number; historyMinutes: number };
}

type GlobalBag = typeof globalThis & {
  __paxPresenceV2?: {
    sessions: Map<string, Session>;
    clients: Map<string, ClientBudget>;
    history: HistorySlot[];
    lastHistoryAt: number;
  };
};

function getStore() {
  const g = globalThis as GlobalBag;
  if (!g.__paxPresenceV2) {
    g.__paxPresenceV2 = {
      sessions: new Map(),
      clients: new Map(),
      history: [],
      lastHistoryAt: 0,
    };
  }
  return g.__paxPresenceV2;
}

function normalizedIp(raw: string | undefined): string | undefined {
  if (!raw || raw.length > 45 || raw.includes("%") || !isIP(raw)) return undefined;
  if (isIP(raw) === 4) return raw;
  const canonical = new URL(`http://[${raw}]/`).hostname.slice(1, -1);
  const mapped = /^::ffff:([a-f0-9]+):([a-f0-9]+)$/.exec(canonical);
  if (!mapped) return canonical;
  const high = parseInt(mapped[1], 16);
  const low = parseInt(mapped[2], 16);
  return `${high >> 8}.${high & 255}.${low >> 8}.${low & 255}`;
}

// Only configured socket peers may supply X-Real-IP; ingress must overwrite it.
const TRUSTED_PROXY_IPS = new Set(
  process.env.PRESENCE_TRUSTED_PROXY_IPS
    ? process.env.PRESENCE_TRUSTED_PROXY_IPS.split(",").map((raw) => {
      const ip = normalizedIp(raw.trim());
      if (!ip) throw new Error("PRESENCE_TRUSTED_PROXY_IPS must contain only IP addresses");
      return ip;
    })
    : [],
);

function clientIp(req: NextApiRequest): string | undefined {
  const peer = normalizedIp(req.socket.remoteAddress);
  if (!peer || !TRUSTED_PROXY_IPS.has(peer)) return peer;
  const verified = req.headers["x-real-ip"];
  return typeof verified === "string" ? normalizedIp(verified) : undefined;
}

function normalizeMarket(raw: unknown): string {
  if (typeof raw !== "string") return "__home__";
  const trimmed = raw.trim().toUpperCase();
  if (!trimmed) return "__home__";
  if (trimmed === "HOME") return "__home__";
  if (!/^[A-Z0-9_/\-]{1,16}$/.test(trimmed)) return "__home__";
  return trimmed;
}

function purge(now: number) {
  const { sessions, clients } = getStore();
  for (const [id, s] of sessions) {
    if (s.identityExpiresAt <= now) {
      sessions.delete(id);
      clients.get(s.client)?.identities.delete(id);
    }
  }
  for (const [client, budget] of clients) {
    if (!budget.identities.size && now - budget.windowStart >= REGISTRATION_WINDOW_MS) {
      clients.delete(client);
    }
  }
}

function snapshotCounts(now: number) {
  const { sessions } = getStore();
  let total = 0;
  const byMarket: Record<string, number> = {};
  for (const [, s] of sessions) {
    if (s.expiresAt <= now) continue;
    total += 1;
    byMarket[s.market] = (byMarket[s.market] ?? 0) + 1;
  }
  return { total, byMarket };
}

function maybeRecordHistory(now: number) {
  const store = getStore();
  if (now - store.lastHistoryAt < HISTORY_INTERVAL_MS) return;
  const counts = snapshotCounts(now);
  store.history.push({ at: now, total: counts.total, byMarket: counts.byMarket });
  while (store.history.length > HISTORY_SLOTS) store.history.shift();
  store.lastHistoryAt = now;
}

function computeSnapshot(now: number): PresenceSnapshot {
  const store = getStore();
  const { total, byMarket } = snapshotCounts(now);

  const past5m = store.history.findLast((slot) => slot.at <= now - 5 * 60_000);

  const entries = Object.entries(byMarket).filter(([m]) => m !== "__home__");

  const top = entries
    .map(([market, count]) => {
      const prev = past5m?.byMarket[market] ?? 0;
      return { market, count, delta5m: count - prev };
    })
    .sort((a, b) => b.count - a.count)
    .slice(0, 8);

  const rising = entries
    .map(([market, count]) => {
      const prev = past5m?.byMarket[market] ?? 0;
      const deltaPct = prev > 0 ? ((count - prev) / prev) * 100 : count > 0 ? 100 : 0;
      return { market, count, deltaPct };
    })
    .filter((r) => r.count >= 2 && r.deltaPct > 0)
    .sort((a, b) => b.deltaPct - a.deltaPct)
    .slice(0, 5);

  return {
    total,
    byMarket,
    top,
    rising,
    history: store.history.map((slot) => ({ at: slot.at, total: slot.total })),
    lastUpdate: new Date(now).toISOString(),
    window: {
      ttlSeconds: Math.round(TTL_MS / 1000),
      historyMinutes: Math.round((HISTORY_SLOTS * HISTORY_INTERVAL_MS) / 60_000),
    },
  };
}

export default function handler(req: NextApiRequest, res: NextApiResponse) {
  res.setHeader("Cache-Control", "no-store, max-age=0");

  const now = Date.now();
  purge(now);

  if (req.method === "POST") {
    const body = (typeof req.body === "string" ? safeJson(req.body) : req.body) ?? {};
    const market = normalizeMarket(body.market);
    const client = clientIp(req);
    if (!client) return res.status(400).json({ error: "Invalid client address" });
    const store = getStore();
    const cookieId = req.cookies[SESSION_COOKIE];
    let existing = cookieId ? store.sessions.get(cookieId) : undefined;
    if (existing && existing.client !== client) {
      return res.status(403).json({ error: "Session belongs to another client" });
    }
    if (!existing) {
      let budget = store.clients.get(client);
      if (budget && now - budget.windowStart >= REGISTRATION_WINDOW_MS) {
        budget.registrations = 0;
        budget.windowStart = now;
      }
      if (
        (budget && (budget.identities.size >= MAX_CLIENT_IDENTITIES || budget.registrations >= MAX_CLIENT_IDENTITIES)) ||
        (!budget && store.clients.size >= MAX_CLIENTS) ||
        store.sessions.size >= MAX_SESSIONS
      ) {
        res.setHeader("Retry-After", String(Math.ceil(IDENTITY_TTL_MS / 1000)));
        return res.status(429).json({ error: "Presence registration limit reached" });
      }
      if (!budget) {
        budget = { identities: new Set(), registrations: 0, windowStart: now };
        store.clients.set(client, budget);
      }
      const sessionId = randomBytes(32).toString("base64url");
      existing = { client, market, expiresAt: now + TTL_MS, identityExpiresAt: now + IDENTITY_TTL_MS, joinedAt: now };
      budget.registrations += 1;
      budget.identities.add(sessionId);
      store.sessions.set(sessionId, existing);
      res.setHeader(
        "Set-Cookie",
        `${SESSION_COOKIE}=${sessionId}; Path=/api/presence; Max-Age=${IDENTITY_TTL_MS / 1000}; HttpOnly; SameSite=Strict${process.env.NODE_ENV === "production" ? "; Secure" : ""}`,
      );
    }
    existing.market = market;
    existing.expiresAt = now + TTL_MS;

    maybeRecordHistory(now);
    return res.status(200).json({ ok: true });
  }

  if (req.method === "GET") {
    maybeRecordHistory(now);
    return res.status(200).json(computeSnapshot(now));
  }

  res.setHeader("Allow", "GET, POST");
  return res.status(405).json({ error: "method not allowed" });
}

function safeJson(raw: string): Record<string, unknown> | null {
  try {
    const v = JSON.parse(raw);
    return v && typeof v === "object" && !Array.isArray(v) ? v : null;
  } catch {
    return null;
  }
}
