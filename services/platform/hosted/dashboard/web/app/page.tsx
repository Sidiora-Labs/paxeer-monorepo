"use client";

import {
  AppShell,
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  List,
  ListItem,
  SectionHeader,
  SegmentedControl,
  SkeletonRow,
  Stat,
  StatPair,
} from "@layerx/ui";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { decodeOverview, decodeRequests, decodeDeliveries, decodeReceipt, type Verification, type KeyView, type RequestRecord, type Endpoint, type Delivery, type Payment, type Overview, type Receipt } from "./data";

import { browserKeyState, createDeveloperKey, registrationProof, sessionProof, type BrowserKeyState } from "./browser-key";

const tabs = [
  { value: "overview", label: "Overview" },
  { value: "keys", label: "Keys" },
  { value: "requests", label: "Logs" },
  { value: "webhooks", label: "Webhooks" },
  { value: "payments", label: "Payments" },
];

class DashboardReadError extends Error {
  readonly status: number;

  constructor(status: number) {
    super(`Dashboard dependency returned ${status}`);
    this.name = "DashboardReadError";
    this.status = status;
  }
}

type DashboardError = { title: string; description: string };

const signInRequired = { title: "Sign in required", description: "A developer session is needed to read this dashboard." };

function verificationBadge(level: Verification) {
  return <Badge size="sm" variant={level === "unverified" ? "warning" : "success"}>{level}</Badge>;
}

async function read<T>(path: string, decode: (value: unknown) => T, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`/v1/dashboard${path}`, { credentials: "include", cache: "no-store", signal });
  if (!response.ok) throw new DashboardReadError(response.status);
  return decode(await response.json());
}

export default function DeveloperDashboard() {
  const [active, setActive] = useState("overview");
  const [overview, setOverview] = useState<Overview>();
  const [requests, setRequests] = useState<RequestRecord[]>([]);
  const [deliveries, setDeliveries] = useState<Delivery[]>([]);
  const [error, setError] = useState<DashboardError>();
  const [receipt, setReceipt] = useState<Receipt>();
  const [needsSession, setNeedsSession] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const readController = useRef<AbortController | undefined>(undefined);

  const clearPanels = useCallback(() => {
    setOverview(undefined);
    setRequests([]);
    setDeliveries([]);
    setReceipt(undefined);
  }, []);

  const refresh = useCallback(async () => {
    readController.current?.abort();
    const controller = new AbortController();
    readController.current = controller;
    const signal = controller.signal;
    try {
      const [summary, requestLog, webhookLog] = await Promise.all([
        read("/overview", decodeOverview, signal),
        read("/requests?limit=100", decodeRequests, signal),
        read("/webhook-deliveries?limit=100", decodeDeliveries, signal),
      ]);
      if (signal.aborted) return;
      setNeedsSession(false);
      setOverview(summary);
      setRequests(requestLog);
      setDeliveries(webhookLog);
      setError(undefined);
    } catch (cause) {
      if (!signal.aborted && (cause as Error).name !== "AbortError") {
        const readError = cause instanceof DashboardReadError ? cause : undefined;
        setError(readError?.status === 401
          ? signInRequired
          : { title: "Dashboard unavailable", description: (cause as Error).message });
        setNeedsSession(readError?.status === 401);
        clearPanels();
      }
    }
  }, [clearPanels]);

  useEffect(() => {
    void refresh();
    return () => readController.current?.abort();
  }, [refresh]);

  const signOut = async () => {
    readController.current?.abort();
    setSigningOut(true);
    try {
      await sessionRequest("/session", {}, "DELETE");
      clearPanels();
      setError(undefined);
      setNeedsSession(true);
    } catch (cause) {
      setError({ title: "Sign-out unavailable", description: refusalWords(cause) });
    } finally {
      setSigningOut(false);
    }
  };
  const signOutControl = !needsSession && overview
    ? <Button type="button" size="sm" variant="secondary" loading={signingOut} onClick={() => void signOut()}>Sign out</Button>
    : undefined;

  const nav = useMemo(() => [
    { id: "home", label: "Overview" },
    { id: "agents", label: "Keys" },
    { id: "activity", label: "Logs", badge: overview?.dead_letters.length },
    { id: "more", label: "Webhooks" },
  ], [overview]);

  return (
    <AppShell
      nav={nav}
      activeNav={active === "overview" ? "home" : active === "keys" ? "agents" : active === "requests" ? "activity" : "more"}
      onNavigate={(value) => setActive(value === "home" ? "overview" : value === "agents" ? "keys" : value === "activity" ? "requests" : "webhooks")}
      title="Developer dashboard"
      notificationControl={signOutControl}
      user={{ name: overview?.principal ?? "Developer", initials: "DX" }}
    >
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-5 p-4 md:p-6">
        {!needsSession ? <SegmentedControl aria-label="Dashboard section" options={tabs} value={active} onValueChange={setActive} /> : null}
        {needsSession ? <SignInView onSignedIn={refresh} /> : null}
        {error && !needsSession ? <Card elevation="outline"><EmptyState title={error.title} description={error.description} /></Card> : null}
        {!overview && !error && !needsSession ? <Card><SkeletonRow /><SkeletonRow /><SkeletonRow /></Card> : null}
        {overview && active === "overview" ? <OverviewPanel value={overview} /> : null}
        {overview && active === "keys" ? <KeysPanel keys={overview.keys} /> : null}
        {overview && active === "requests" ? <RequestsPanel requests={requests} /> : null}
        {overview && active === "webhooks" ? <WebhooksPanel endpoints={overview.endpoints} deliveries={deliveries} /> : null}
        {overview && active === "payments" ? <PaymentsPanel payments={overview.payments} onReceipt={async (activity) => {
          try {
            const controller = new AbortController();
            readController.current?.abort();
            readController.current = controller;
            const value = await read(`/receipts/${encodeURIComponent(activity)}`, decodeReceipt, controller.signal);
            if (controller.signal.aborted) return;
            if (value.activity_id !== activity) throw new Error("Receipt activity mismatch");
            setReceipt(value);
            setError(undefined);
          } catch (cause) {
            if ((cause as Error).name === "AbortError") return;
            setReceipt(undefined);
            const readError = cause instanceof DashboardReadError ? cause : undefined;
            if (readError?.status === 401) {
              clearPanels();
              setNeedsSession(true);
            }
            setError(readError?.status === 401
              ? signInRequired
              : { title: "Dashboard unavailable", description: (cause as Error).message });
          }
        }} /> : null}
        {receipt && active === "payments" ? <Card><SectionHeader title="Verified receipt" /><List><ListItem title={receipt.activity_id} subtitle={`${receipt.event} · ${receipt.receipt_digest}`} trailing={verificationBadge(receipt.verification)} /></List></Card> : null}
      </div>
    </AppShell>
  );
}

class SignInFailure extends Error {}

async function sessionRequest(path: string, body: object, method = "POST"): Promise<void> {
  let result: Response;
  try {
    result = await fetch(path, {
      method,
      credentials: "same-origin",
      cache: "no-store",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    throw new SignInFailure("connection_unavailable");
  }
  if (!result.ok) {
    let reason = "sign_in_unavailable";
    try {
      const value = await result.json();
      if (typeof value.reason === "string" && /^[a-z][a-z0-9_]{0,95}$/.test(value.reason)) reason = value.reason;
    } catch {
      throw new SignInFailure(reason);
    }
    throw new SignInFailure(reason);
  }
}

function refusalWords(cause: unknown): string {
  const reason = cause instanceof Error ? cause.message : "sign_in_unavailable";
  const messages: Record<string, string> = {
    sign_in_not_configured: "Key sign-in is not configured. Use a session token.",
    session_proof_invalid: "The session proof was not accepted.",
    registration_proof_invalid: "The registration proof was not accepted. Check the gateway tenant.",
    session_time_out_of_range: "The proof time is outside the accepted range. Check this device’s clock.",
    session_proof_replay: "This proof was already used. Wait a second and sign in again.",
    session_required: "The session token was not accepted.",
    invalid_token: "Enter a valid session token.",
    invalid_tenant: "Enter the tenant configured by your gateway operator.",
    browser_key_storage_unavailable: "Browser key storage is unavailable. Use a session token.",
    browser_key_missing: "This browser has no developer key.",
  };
  if (messages[reason]) return messages[reason];
  if (/^[a-z][a-z0-9_]{0,95}$/.test(reason)) {
    const words = reason.replaceAll("_", " ");
    return `${words[0].toUpperCase()}${words.slice(1)}.`;
  }
  return "Sign-in is unavailable. Please try again.";
}

function SignInView({ onSignedIn }: { onSignedIn: () => Promise<void> }) {
  const [keyState, setKeyState] = useState<BrowserKeyState>();
  const [tenant, setTenant] = useState("");
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let active = true;
    void browserKeyState().then((state) => { if (active) setKeyState(state); }).catch(() => {
      if (active) {
        setKeyState({ ed25519: false, hasKey: false });
        setMessage("Browser key storage is unavailable. Use a session token.");
      }
    });
    return () => { active = false; };
  }, []);

  const withKey = async (create: boolean) => {
    setBusy(true);
    setMessage("");
    try {
      if (!/^[a-z0-9._-]{1,95}$/.test(tenant)) throw new SignInFailure("invalid_tenant");
      if (create) {
        const publicKey = await createDeveloperKey();
        setKeyState({ ed25519: true, hasKey: true, publicKey });
      }
      await sessionRequest("/session/registration", await registrationProof(tenant));
      await sessionRequest("/session", await sessionProof(tenant));
      await onSignedIn();
    } catch (cause) {
      setMessage(refusalWords(cause));
    } finally {
      setBusy(false);
    }
  };

  return <Card className="flex flex-col gap-4">
    <SectionHeader title="Sign in to your developer dashboard" />
    {keyState === undefined ? <SkeletonRow /> : null}
    {keyState?.ed25519 ? <>
      <label htmlFor="gateway-tenant">Gateway tenant</label>
      <Input id="gateway-tenant" value={tenant} onChange={(event) => setTenant(event.target.value)} disabled={busy} autoCapitalize="none" autoCorrect="off" spellCheck={false} maxLength={95} aria-describedby="tenant-help" />
      <p id="tenant-help">Use the tenant configured by your gateway operator.</p>
      <p>The private developer key stays in this browser.</p>
      {keyState.publicKey ? <div className="flex flex-col gap-2">
        <span>Public key</span><code className="break-all">{keyState.publicKey}</code>
        <Button type="button" size="sm" variant="secondary" onClick={async () => {
          try {
            await navigator.clipboard.writeText(keyState.publicKey!);
            setCopied(true);
          } catch {
            setMessage("The public key could not be copied.");
          }
        }}>{copied ? "Public key copied" : "Copy public key"}</Button>
      </div> : null}
      <Button type="button" className="whitespace-normal" disabled={busy || !tenant || keyState.hasKey} onClick={() => void withKey(true)}>Create a developer key in this browser and sign in</Button>
      <Button type="button" className="whitespace-normal" variant="secondary" disabled={busy || !tenant || !keyState.hasKey} onClick={() => void withKey(false)}>Sign in with the key this browser holds</Button>
    </> : keyState ? <p>This browser does not support Ed25519 keys; sign in with a session token.</p> : null}
    <form className="flex flex-col gap-3" onSubmit={(event) => {
      event.preventDefault();
      const sessionToken = token;
      setToken("");
      setBusy(true);
      setMessage("");
      void sessionRequest("/session", { token: sessionToken }).then(onSignedIn).catch((cause) => setMessage(refusalWords(cause))).finally(() => setBusy(false));
    }}>
      <label htmlFor="session-token">Session token</label>
      <Input id="session-token" type="password" autoComplete="off" spellCheck={false} value={token} onChange={(event) => setToken(event.target.value)} disabled={busy} maxLength={4096} required />
      <Button type="submit" variant="secondary" loading={busy} disabled={!token}>Sign in with a session token</Button>
    </form>
    {message ? <p role="alert">{message}</p> : null}
  </Card>;
}

function OverviewPanel({ value }: { value: Overview }) {
  return <>
    <Card><StatPair left={{ value: value.usage.requests_used, label: "Requests used" }} right={{ value: value.usage.requests_remaining, label: "Requests remaining" }} /></Card>
    <div className="grid gap-4 md:grid-cols-3">
      <Card><Stat value={value.usage.live_keys} label="Live API keys" /></Card>
      <Card><Stat value={value.endpoints.length} label="Webhook endpoints" /></Card>
      <Card><Stat value={value.payments.filter((payment) => payment.settled).length} label="Verified test payments" /></Card>
    </div>
    <Card><SectionHeader title="Recent protocol activity" /><List>{value.recent_requests.slice(0, 8).map((request) => <ListItem key={`${request.at}-${request.operation_digest}`} title={request.outcome} subtitle={request.operation_digest} trailing={verificationBadge(request.verification)} trailingCaption={new Date(request.at * 1000).toLocaleString()} />)}</List></Card>
  </>;
}

function KeysPanel({ keys }: { keys: KeyView[] }) {
  return <Card><SectionHeader title="API keys and current quota windows" />{keys.length === 0 ? <EmptyState title="No API keys" description="Issue a principal-bound key through the gateway key management surface." /> : <List>{keys.map((key) => <ListItem key={key.key_id} title={key.key_id} subtitle={`${key.used_in_window} of ${key.requests_per_window} requests used · ${key.window_seconds}s window`} trailing={<Badge variant={key.disabled ? "destructive" : "success"}>{key.disabled ? "revoked" : "active"}</Badge>} trailingCaption={`${key.remaining_in_window} remaining`} />)}</List>}</Card>;
}

function RequestsPanel({ requests }: { requests: RequestRecord[] }) {
  return <Card><SectionHeader title="Principal-isolated request log" /><List>{requests.map((request) => <ListItem key={`${request.at}-${request.operation_digest}`} title={request.outcome} subtitle={request.operation_digest} trailing={verificationBadge(request.verification)} trailingCaption={new Date(request.at * 1000).toLocaleString()} />)}</List></Card>;
}

function WebhooksPanel({ endpoints, deliveries }: { endpoints: Endpoint[]; deliveries: Delivery[] }) {
  return <div className="grid gap-5 lg:grid-cols-2"><Card><SectionHeader title="Webhook endpoints" /><List>{endpoints.map((endpoint) => <ListItem key={endpoint.endpoint} title={endpoint.url} subtitle={`${endpoint.pending} pending · ${endpoint.in_flight} in flight · ${endpoint.retrying} retrying`} trailing={<Badge variant={endpoint.suspended ? "destructive" : "success"}>{endpoint.suspended ? "suspended" : "active"}</Badge>} trailingCaption={`${endpoint.delivered_total} delivered / ${endpoint.dead_lettered_total} dead-lettered`} />)}</List></Card><Card><SectionHeader title="Delivery log" /><List>{deliveries.map((delivery) => <ListItem key={delivery.delivery} title={delivery.event} subtitle={`${delivery.endpoint} · ${delivery.delivery}`} trailing={verificationBadge(delivery.verification)} trailingCaption={delivery.state.state} />)}</List></Card></div>;
}

function PaymentsPanel({ payments, onReceipt }: { payments: Payment[]; onReceipt: (activity: string) => Promise<void> }) {
  return <Card><SectionHeader title="Real test payments and verified receipts" />{payments.length === 0 ? <EmptyState title="No test payments" description="Payments appear only after the canonical payment source and independent receipt authority agree." /> : <List>{payments.map((payment) => <ListItem key={payment.event} title={`${payment.amount ?? "—"} ${payment.asset ?? ""}`} subtitle={`${payment.event} · ${payment.facts.map((fact) => `${fact.name}=${fact.value} [${fact.verification}]`).join(" · ")} · receipt ${payment.receipt_digest ?? "not available"} · all facts ${payment.verification}`} trailing={<div>{verificationBadge(payment.settlement_verification)}{payment.settled ? payment.facts.filter((fact) => fact.name === "activity_id" && fact.verification !== "unverified" && fact.receipt_digest === payment.receipt_digest).map((fact) => <button key={fact.value} type="button" onClick={() => void onReceipt(fact.value)}>View receipt</button>) : null}</div>} trailingCaption={payment.settled ? "settlement receipt verified" : "settlement not verified"} />)}</List>}</Card>;
}
