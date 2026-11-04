"use client";

// NetworkPill — live chain status indicator for header slots.
// Port of src/components/design/NetworkPill.jsx, rewired to real wagmi state.
// Shows the active chain (Paxeer only in this build), live block number, and
// measured public feed latency (status endpoint round-trip). Signal bars reflect
// latency buckets. Click to expand a details popover.

import {
  useEffect,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { ChevronDown } from "lucide-react";
import { useChainId } from "wagmi";
import { paxeerChain } from "@/core/chain";
import { FEED_ORIGIN } from "@/core/feeds";
import { fetchJsonWithPolicy } from "@/core/net/fetch";
import { activePoll } from "@/core/net/realtime";

function latencyToBars(ms: number | null): 0 | 1 | 2 | 3 | 4 {
  if (ms === null) return 0;
  if (ms < 60) return 4;
  if (ms < 100) return 3;
  if (ms < 150) return 2;
  return 1;
}

export interface NetworkStatus {
  blockNumber: number | null;
  latency: number | null;
  observedAt: number | null;
  stale: boolean;
}

export const NETWORK_STATUS_MAX_AGE_MS = 60_000;

// Public status only. No operator telemetry socket or background RPC poll.
export class NetworkStatusMonitor {
  private status: NetworkStatus = {
    blockNumber: null,
    latency: null,
    observedAt: null,
    stale: true,
  };
  private generation = 0;
  private stopPoll: (() => void) | undefined;
  private expiry: ReturnType<typeof setTimeout> | undefined;

  constructor(
    private readonly onChange: (status: NetworkStatus) => void,
    private readonly healthUrl = `${FEED_ORIGIN}/api/explorer/health`,
  ) {}

  getSnapshot(): NetworkStatus {
    return this.status;
  }

  private publish(status: NetworkStatus) {
    if (this.expiry !== undefined) clearTimeout(this.expiry);
    this.expiry = undefined;
    this.status = status;
    this.onChange(status);
    if (!status.stale && status.observedAt !== null) {
      const expiresAt = status.observedAt + NETWORK_STATUS_MAX_AGE_MS;
      this.expiry = setTimeout(
        () => {
          if (Date.now() >= expiresAt) this.markStale();
          else this.publish(this.status);
        },
        Math.max(0, expiresAt - Date.now()),
      );
    }
  }

  private markStale() {
    this.publish({ ...this.status, latency: null, stale: true });
  }

  start() {
    this.stop();
    if (
      this.status.observedAt !== null &&
      Date.now() - this.status.observedAt >= NETWORK_STATUS_MAX_AGE_MS
    ) {
      this.markStale();
    } else {
      this.publish(this.status);
    }
    const generation = this.generation;
    this.stopPoll = activePoll(async () => {
      const started = Date.now();
      try {
        const data = await fetchJsonWithPolicy<{
          latest_block: number | null;
          chain_connected: boolean;
        }>(this.healthUrl);
        if (generation !== this.generation) return;
        if (!data.chain_connected) {
          this.markStale();
          return;
        }
        const observedAt = Date.now();
        this.publish({
          blockNumber: data.latest_block,
          latency: observedAt - started,
          observedAt,
          stale: false,
        });
      } catch (error) {
        if (generation === this.generation) this.markStale();
        throw error;
      }
    }, 30_000);
  }

  stop() {
    this.generation++;
    this.stopPoll?.();
    this.stopPoll = undefined;
  }

  dispose() {
    this.stop();
    if (this.expiry !== undefined) clearTimeout(this.expiry);
    this.expiry = undefined;
  }
}

export function networkBlockLabel(status: NetworkStatus): string {
  if (status.blockNumber === null) return "…";
  return `#${status.blockNumber.toLocaleString()}${status.stale ? " (stale)" : ""}`;
}

function useNetworkStatus(open: boolean): NetworkStatus {
  const [status, setStatus] = useState<NetworkStatus>({
    blockNumber: null,
    latency: null,
    observedAt: null,
    stale: true,
  });
  const [monitor] = useState(() => new NetworkStatusMonitor(setStatus));
  useEffect(() => () => monitor.dispose(), [monitor]);
  useEffect(() => {
    if (!open) return;
    monitor.start();
    return () => monitor.stop();
  }, [monitor, open]);
  return status;
}

export interface NetworkPillProps {
  className?: string;
  style?: CSSProperties;
}

const PILL_BUTTON: CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 10,
  background: "var(--bg-subtle)",
  border: "1px solid var(--border)",
  borderRadius: 999,
  padding: "8px 14px 8px 10px",
  color: "var(--fg)",
  fontFamily: "var(--font-body)",
  fontSize: 13,
  fontWeight: 500,
  cursor: "pointer",
};

const BAR_HEIGHTS = [4, 7, 10, 12] as const;

function SignalBars({ bars }: { bars: 0 | 1 | 2 | 3 | 4 }) {
  return (
    <span style={{ display: "flex", alignItems: "flex-end", gap: 1.5, height: 12 }}>
      {BAR_HEIGHTS.map((h, i) => (
        <span
          key={i}
          style={{
            width: 2,
            height: h,
            borderRadius: 1,
            background: i < bars ? "var(--success)" : "rgba(255,255,255,0.12)",
          }}
        />
      ))}
    </span>
  );
}

function shortRpc(rpc: string): string {
  try {
    const u = new URL(rpc);
    return u.host;
  } catch {
    return rpc;
  }
}

export function NetworkPill({ className, style }: NetworkPillProps) {
  const chainId = useChainId();
  const [open, setOpen] = useState(false);
  const status = useNetworkStatus(open);
  const { latency } = status;
  const ref = useRef<HTMLDivElement | null>(null);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (!ref.current) return;
      if (!ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const isPaxeer = chainId === paxeerChain.id;
  const color = isPaxeer ? "var(--brand-blue)" : "var(--fg-muted)";
  const name = isPaxeer ? "Paxeer" : `Chain ${chainId}`;
  const rpcUrl = paxeerChain.rpcUrls.default.http[0] ?? "";
  const bars = latencyToBars(latency);

  return (
    <div
      ref={ref}
      className={className}
      style={{ position: "relative", display: "inline-block", ...style }}
    >
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        style={PILL_BUTTON}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={`Network ${name}, latency ${latency === null ? "unknown" : `${Math.round(latency)} ms`}`}
      >
        <span
          style={{
            width: 20,
            height: 20,
            borderRadius: 999,
            background: color,
            flexShrink: 0,
            boxShadow: isPaxeer ? "0 0 0 3px rgba(0,76,237,0.15)" : "none",
          }}
        />
        <span>{name}</span>
        <SignalBars bars={bars} />
        <span
          style={{
            fontFamily: "var(--font-mono)",
            fontSize: 10,
            color: "var(--fg-subtle)",
            paddingLeft: 4,
            borderLeft: "1px solid var(--border)",
            minWidth: 44,
            textAlign: "right",
          }}
        >
          {latency === null ? "…ms" : `${Math.round(latency)}ms`}
        </span>
        <ChevronDown
          width={14}
          height={14}
          style={{
            transform: open ? "rotate(180deg)" : "none",
            transition: "transform 200ms",
            color: "var(--fg-muted)",
          }}
        />
      </button>

      {open && (
        <div
          role="dialog"
          aria-label="Network details"
          style={{
            position: "absolute",
            top: "calc(100% + 6px)",
            right: 0,
            width: 272,
            padding: 6,
            background: "var(--bg-elevated)",
            border: "1px solid var(--border-strong)",
            borderRadius: 12,
            boxShadow: "var(--shadow-05)",
            zIndex: 50,
          }}
        >
          <div style={{ padding: "10px 12px 12px 12px" }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                paddingBottom: 10,
                borderBottom: "1px solid var(--border)",
              }}
            >
              <span
                style={{
                  width: 16,
                  height: 16,
                  borderRadius: 999,
                  background: color,
                  flexShrink: 0,
                }}
              />
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--fg)" }}>
                  {name}
                </div>
                <div
                  style={{
                    fontFamily: "var(--font-mono)",
                    fontSize: 10,
                    color: "var(--fg-subtle)",
                  }}
                >
                  chain {chainId}
                </div>
              </div>
              <SignalBars bars={bars} />
            </div>
            <dl
              style={{
                display: "grid",
                gridTemplateColumns: "auto 1fr",
                gap: "8px 12px",
                margin: 0,
                marginTop: 10,
              }}
            >
              <dt
                style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 9,
                  letterSpacing: "0.14em",
                  textTransform: "uppercase",
                  color: "var(--fg-subtle)",
                }}
              >
                Block
              </dt>
              <dd
                style={{
                  margin: 0,
                  fontFamily: "var(--font-mono)",
                  fontSize: 11,
                  color: "var(--fg)",
                  textAlign: "right",
                }}
              >
                {networkBlockLabel(status)}
              </dd>

              <dt
                style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 9,
                  letterSpacing: "0.14em",
                  textTransform: "uppercase",
                  color: "var(--fg-subtle)",
                }}
              >
                Feed latency
              </dt>
              <dd
                style={{
                  margin: 0,
                  fontFamily: "var(--font-mono)",
                  fontSize: 11,
                  color:
                    bars >= 3
                      ? "var(--success)"
                      : bars === 2
                        ? "var(--warning)"
                        : "var(--danger)",
                  textAlign: "right",
                }}
              >
                {latency === null ? "—" : `${Math.round(latency)} ms`}
              </dd>

              <dt
                style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 9,
                  letterSpacing: "0.14em",
                  textTransform: "uppercase",
                  color: "var(--fg-subtle)",
                }}
              >
                RPC
              </dt>
              <dd
                style={{
                  margin: 0,
                  fontFamily: "var(--font-mono)",
                  fontSize: 11,
                  color: "var(--fg-muted)",
                  textAlign: "right",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}
                title={rpcUrl}
              >
                {shortRpc(rpcUrl)}
              </dd>
            </dl>
          </div>
        </div>
      )}
    </div>
  );
}

export default NetworkPill;
