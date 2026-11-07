import Link from "next/link";
import { useEffect, useState } from "react";

const STATUS_PAGE_ENV = "NEXT_PUBLIC_STATUS_PAGE_URL";

export function parseStatusPageUrl(raw: string | undefined): string | null {
  if (!raw) return null;
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new Error(`${STATUS_PAGE_ENV} is not a valid URL: ${raw}`);
  }
  if (url.protocol !== "https:") throw new Error(`${STATUS_PAGE_ENV} must use https: ${raw}`);
  if (url.origin !== raw) throw new Error(`${STATUS_PAGE_ENV} must be a bare origin with no trailing slash: ${raw}`);
  return raw;
}

const STATUS_PAGE_URL = parseStatusPageUrl(process.env.NEXT_PUBLIC_STATUS_PAGE_URL);

export type FooterHealth = {
  state: "unknown" | "confirmed" | "unavailable" | "stale";
  description: string;
  indicator: "none" | "minor" | "major" | "critical" | "maintenance" | null;
};

const UNKNOWN_HEALTH: FooterHealth = {
  state: "unknown",
  description: "Status unknown",
  indicator: null,
};

export async function loadFooterStatus(statusPage: string, signal: AbortSignal): Promise<FooterHealth> {
  const response = await fetch(`${statusPage}/api/v2/status.json`, { signal, cache: "no-store" });
  if (!response.ok) throw new Error("Status request failed");
  const body: unknown = await response.json();
  if (typeof body !== "object" || body === null || !("status" in body)) {
    throw new Error("Invalid status response");
  }
  const status = body.status;
  if (typeof status !== "object" || status === null || !("indicator" in status) || !("description" in status)) {
    throw new Error("Invalid status response");
  }
  const { indicator, description } = status;
  if (
    (indicator !== "none" && indicator !== "minor" && indicator !== "major" && indicator !== "critical" && indicator !== "maintenance") ||
    typeof description !== "string" || !description.trim()
  ) {
    throw new Error("Invalid status response");
  }
  return { state: "confirmed", description, indicator };
}

export function watchFooterStatus(
  statusPage: string,
  publish: (health: FooterHealth) => void,
  { timeoutMs = 5000, refreshMs = 60000 } = {},
): () => void {
  let stopped = false;
  let confirmed = false;
  let refresh: ReturnType<typeof setTimeout> | undefined;
  let timeout: ReturnType<typeof setTimeout> | undefined;
  let controller: AbortController;
  publish(UNKNOWN_HEALTH);
  const poll = async () => {
    controller = new AbortController();
    let expired = false;
    const unavailable = () => publish({
      state: confirmed ? "stale" : "unavailable",
      description: confirmed ? "Status stale" : "Status unavailable", indicator: null,
    });
    timeout = setTimeout(() => {
      expired = true;
      controller.abort();
      if (!stopped) unavailable();
    }, timeoutMs);
    try {
      const health = await loadFooterStatus(statusPage, controller.signal);
      if (!stopped && !expired) {
        confirmed = true;
        publish(health);
      }
    } catch {
      if (!stopped && !expired) unavailable();
    } finally {
      clearTimeout(timeout);
      if (!stopped) refresh = setTimeout(() => void poll(), refreshMs);
    }
  };
  void poll();
  return () => {
    stopped = true;
    clearTimeout(timeout);
    clearTimeout(refresh);
    controller.abort();
  };
}

export function FooterStatus({ statusPage, health }: { statusPage: string; health: FooterHealth }) {
  return (
    <a
      href={statusPage}
      target="_blank"
      rel="noopener noreferrer"
      className="flex items-center gap-1.5 hover:text-foreground transition-colors"
    >
      <span aria-hidden="true" className={`inline-block w-1.5 h-1.5 rounded-full ${health.state !== "confirmed" ? "bg-muted-foreground" : health.indicator === "none" ? "bg-emerald-500" : "bg-amber-500"}`} />
      <span role="status" data-health={health.state}>{health.description}</span>
    </a>
  );
}

export function Footer() {
  const statusPage = STATUS_PAGE_URL;
  const [health, setHealth] = useState<FooterHealth>(UNKNOWN_HEALTH);
  useEffect(() => {
    if (statusPage) return watchFooterStatus(statusPage, setHealth);
  }, [statusPage]);
  return (
    <>
      <footer className="border-t border-border py-1.5 px-4 flex items-center justify-between text-[10px] text-muted-foreground">
        {statusPage ? (
          <FooterStatus statusPage={statusPage} health={health} />
        ) : (
          <span />
        )}

        <div className="flex items-center gap-3 flex-wrap justify-end">
          <a
            href="https://t.me/paxeernetwork"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-foreground transition-colors"
          >
            Join our community
          </a>
          <span className="text-muted-foreground/40">·</span>
          <Link href="/privacy" className="hover:text-foreground transition-colors">
            Privacy Policy
          </Link>
          <span className="text-muted-foreground/40">·</span>
          <Link href="/terms" className="hover:text-foreground transition-colors">
            Terms of Service
          </Link>
          <span className="text-muted-foreground/40">·</span>
          <Link href="/cookies" className="hover:text-foreground transition-colors">
            Cookies
          </Link>
          <span className="text-muted-foreground/40">·</span>
          <span>Charts by <span className="font-medium text-foreground/70">TradingView</span></span>
        </div>
      </footer>
    </>
  );
}
