"use client";

import {
  Avatar,
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  List,
  ListItem,
  SkeletonRow,
  Stat,
  StatPair,
} from "@layerx/ui";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

import {
  decodeOverview,
  decodeRequests,
  decodeDeliveries,
  decodeReceipt,
  type Verification,
  type KeyView,
  type RequestRecord,
  type Endpoint,
  type Delivery,
  type Payment,
  type Overview,
  type Receipt,
} from "./data";

import {
  browserKeyState,
  createDeveloperKey,
  registrationProof,
  sessionProof,
  type BrowserKeyState,
} from "./browser-key";

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

function DashboardShell({
  principal,
  notificationControl,
  children,
}: {
  principal: string;
  notificationControl?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="dashboard-shell">
      <header className="dashboard-header">
        <span className="dashboard-brand" role="img" aria-label="LayerX">
          LX
        </span>
        <h1>Developer dashboard</h1>
        <div className="dashboard-account">
          <Avatar alt={principal} initials="DX" size="sm" tone="primary" />
          <span>{principal}</span>
        </div>
        {notificationControl}
      </header>
      <main>{children}</main>
    </div>
  );
}

function DashboardLoading() {
  const placeholder = <span className="dashboard-number-skeleton">0</span>;
  return (
    <div className="dashboard-panel dashboard-loading" aria-busy="true">
      <progress className="dashboard-progress" aria-label="Loading dashboard" />
      <Card className="dashboard-card" aria-hidden="true">
        <StatPair
          className="dashboard-usage"
          left={{ value: placeholder, label: "Requests used" }}
          right={{ value: placeholder, label: "Requests remaining" }}
        />
      </Card>
      <div className="dashboard-stats" aria-hidden="true">
        <Card className="dashboard-card">
          <Stat value={placeholder} label="Live API keys" />
        </Card>
        <Card className="dashboard-card">
          <Stat value={placeholder} label="Webhook endpoints" />
        </Card>
        <Card className="dashboard-card">
          <Stat value={placeholder} label="Verified test payments" />
        </Card>
      </div>
      <Card className="dashboard-card" aria-hidden="true">
        <h2>Recent protocol activity</h2>
        <SkeletonRow />
        <SkeletonRow />
        <SkeletonRow />
      </Card>
    </div>
  );
}

function DashboardTabs({
  active,
  onChange,
}: {
  active: string;
  onChange: (value: string) => void;
}) {
  const selectedTab = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    selectedTab.current?.scrollIntoView({
      block: "nearest",
      inline: "nearest",
    });
  }, [active]);

  return (
    <div
      className="dashboard-tabs"
      role="tablist"
      aria-label="Dashboard section"
    >
      {tabs.map((tab, index) => (
        <Button
          key={tab.value}
          ref={active === tab.value ? selectedTab : undefined}
          id={`tab-${tab.value}`}
          type="button"
          role="tab"
          variant="ghost"
          aria-selected={active === tab.value}
          tabIndex={active === tab.value ? 0 : -1}
          onClick={() => onChange(tab.value)}
          onKeyDown={(event) => {
            let next: number;
            const rtl =
              getComputedStyle(event.currentTarget).direction === "rtl";
            if (event.key === "ArrowRight")
              next = (index + (rtl ? -1 : 1) + tabs.length) % tabs.length;
            else if (event.key === "ArrowLeft")
              next = (index + (rtl ? 1 : -1) + tabs.length) % tabs.length;
            else if (event.key === "Home") next = 0;
            else if (event.key === "End") next = tabs.length - 1;
            else return;
            event.preventDefault();
            onChange(tabs[next].value);
            document.getElementById(`tab-${tabs[next].value}`)?.focus();
          }}
        >
          {tab.label}
        </Button>
      ))}
    </div>
  );
}

function DashboardCard({
  title,
  children,
  className = "",
}: {
  title: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Card className={`dashboard-card ${className}`}>
      <h2>{title}</h2>
      {children}
    </Card>
  );
}

const signInRequired = {
  title: "Sign in required",
  description: "A developer session is needed to read this dashboard.",
};

function verificationBadge(level: Verification) {
  return (
    <Badge
      className="dashboard-badge"
      size="sm"
      variant={level === "unverified" ? "warning" : "success"}
    >
      {level}
    </Badge>
  );
}

async function read<T>(
  path: string,
  decode: (value: unknown) => T,
  signal?: AbortSignal,
): Promise<T> {
  const response = await fetch(`/v1/dashboard${path}`, {
    credentials: "include",
    cache: "no-store",
    signal,
  });
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
  const [receiptActivity, setReceiptActivity] = useState<string>();
  const receiptHeading = useRef<HTMLHeadingElement | null>(null);
  const [needsSession, setNeedsSession] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const readController = useRef<AbortController | undefined>(undefined);
  const previousNeedsSession = useRef(false);

  const clearPanels = useCallback(() => {
    setOverview(undefined);
    setRequests([]);
    setDeliveries([]);
    setReceipt(undefined);
    setReceiptActivity(undefined);
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
        const readError =
          cause instanceof DashboardReadError ? cause : undefined;
        setError(
          readError?.status === 401
            ? signInRequired
            : {
                title: "Dashboard unavailable",
                description: (cause as Error).message,
              },
        );
        setNeedsSession(readError?.status === 401);
        clearPanels();
      }
    }
  }, [clearPanels]);

  useEffect(() => {
    void refresh();
    return () => readController.current?.abort();
  }, [refresh]);

  useEffect(() => {
    if (receipt) receiptHeading.current?.focus();
  }, [receipt]);

  useEffect(() => {
    if (previousNeedsSession.current && !needsSession) {
      document.getElementById(`tab-${active}`)?.focus();
    }
    previousNeedsSession.current = needsSession;
  }, [needsSession, active]);

  const signOut = async () => {
    readController.current?.abort();
    setSigningOut(true);
    try {
      await sessionRequest("/session", {}, "DELETE");
      clearPanels();
      setError(undefined);
      setNeedsSession(true);
    } catch (cause) {
      setError({
        title: "Sign-out unavailable",
        description: refusalWords(cause),
      });
    } finally {
      setSigningOut(false);
    }
  };
  const signOutControl =
    !needsSession && overview ? (
      <Button
        type="button"
        variant="secondary"
        loading={signingOut}
        onClick={() => void signOut()}
      >
        Sign out
      </Button>
    ) : undefined;

  return (
    <DashboardShell
      notificationControl={signOutControl}
      principal={overview?.principal ?? "Developer"}
    >
      <div className="dashboard-content">
        {!needsSession ? (
          <DashboardTabs active={active} onChange={setActive} />
        ) : null}
        {needsSession ? <SignInView onSignedIn={refresh} /> : null}
        {error && !needsSession ? (
          <DashboardCard title={error.title}>
            <p role="alert">{error.description}</p>
          </DashboardCard>
        ) : null}
        {!overview && !error && !needsSession ? <DashboardLoading /> : null}
        {overview ? (
          <div
            id={`panel-${active}`}
            className="dashboard-panel"
            role="tabpanel"
            aria-labelledby={`tab-${active}`}
            tabIndex={0}
          >
            {overview && active === "overview" ? (
              <OverviewPanel value={overview} />
            ) : null}
            {overview && active === "keys" ? (
              <KeysPanel keys={overview.keys} />
            ) : null}
            {overview && active === "requests" ? (
              <RequestsPanel requests={requests} />
            ) : null}
            {overview && active === "webhooks" ? (
              <WebhooksPanel
                endpoints={overview.endpoints}
                deliveries={deliveries}
              />
            ) : null}
            {overview && active === "payments" ? (
              <PaymentsPanel
                payments={overview.payments}
                receiptActivity={receiptActivity}
                onReceipt={async (activity) => {
                  setReceiptActivity(activity);
                  setReceipt(undefined);
                  try {
                    const controller = new AbortController();
                    readController.current?.abort();
                    readController.current = controller;
                    const value = await read(
                      `/receipts/${encodeURIComponent(activity)}`,
                      decodeReceipt,
                      controller.signal,
                    );
                    if (controller.signal.aborted) return;
                    if (value.activity_id !== activity)
                      throw new Error("Receipt activity mismatch");
                    setReceipt(value);
                    setError(undefined);
                  } catch (cause) {
                    if ((cause as Error).name === "AbortError") return;
                    setReceipt(undefined);
                    const readError =
                      cause instanceof DashboardReadError ? cause : undefined;
                    if (readError?.status === 401) {
                      clearPanels();
                      setNeedsSession(true);
                    }
                    setError(
                      readError?.status === 401
                        ? signInRequired
                        : {
                            title: "Dashboard unavailable",
                            description: (cause as Error).message,
                          },
                    );
                  } finally {
                    setReceiptActivity(undefined);
                  }
                }}
              />
            ) : null}
            {receipt && active === "payments" ? (
              <Card className="dashboard-card">
                <h2 ref={receiptHeading} tabIndex={-1}>
                  Verified receipt
                </h2>
                <List>
                  <ListItem
                    title={receipt.activity_id}
                    subtitle={`${receipt.event} · ${receipt.receipt_digest}`}
                    trailing={verificationBadge(receipt.verification)}
                  />
                </List>
              </Card>
            ) : null}
          </div>
        ) : null}
      </div>
    </DashboardShell>
  );
}

class SignInFailure extends Error {}

async function sessionRequest(
  path: string,
  body: object,
  method = "POST",
): Promise<void> {
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
      if (
        typeof value.reason === "string" &&
        /^[a-z][a-z0-9_]{0,95}$/.test(value.reason)
      )
        reason = value.reason;
    } catch {
      throw new SignInFailure(reason);
    }
    throw new SignInFailure(reason);
  }
}

function refusalWords(cause: unknown): string {
  const reason = cause instanceof Error ? cause.message : "sign_in_unavailable";
  const messages: Record<string, string> = {
    sign_in_not_configured:
      "Key sign-in is not configured. Use a session token.",
    session_proof_invalid: "The session proof was not accepted.",
    registration_proof_invalid:
      "The registration proof was not accepted. Check the gateway tenant.",
    session_time_out_of_range:
      "The proof time is outside the accepted range. Check this device’s clock.",
    session_proof_replay:
      "This proof was already used. Wait a second and sign in again.",
    session_required: "The session token was not accepted.",
    invalid_token: "Enter a valid session token.",
    invalid_tenant: "Enter the tenant configured by your gateway operator.",
    browser_key_storage_unavailable:
      "Browser key storage is unavailable. Use a session token.",
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
  const [busy, setBusy] = useState<"create" | "key" | "token">();
  const [tenantInvalid, setTenantInvalid] = useState(false);
  const [tokenInvalid, setTokenInvalid] = useState(false);
  const signInHeading = useRef<HTMLHeadingElement | null>(null);
  const [message, setMessage] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    signInHeading.current?.focus();
    let active = true;
    void browserKeyState()
      .then((state) => {
        if (active) setKeyState(state);
      })
      .catch(() => {
        if (active) {
          setKeyState({ ed25519: false, hasKey: false });
          setMessage(
            "Browser key storage is unavailable. Use a session token.",
          );
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const withKey = async (create: boolean) => {
    setBusy(create ? "create" : "key");
    setMessage("");
    try {
      if (!/^[a-z0-9._-]{1,95}$/.test(tenant))
        throw new SignInFailure("invalid_tenant");
      if (create) {
        const publicKey = await createDeveloperKey();
        setKeyState({ ed25519: true, hasKey: true, publicKey });
      }
      await sessionRequest(
        "/session/registration",
        await registrationProof(tenant),
      );
      await sessionRequest("/session", await sessionProof(tenant));
      await onSignedIn();
    } catch (cause) {
      setMessage(refusalWords(cause));
    } finally {
      setBusy(undefined);
    }
  };

  return (
    <Card className="dashboard-card dashboard-sign-in">
      <h2 ref={signInHeading} tabIndex={-1}>
        Sign in to your developer dashboard
      </h2>
      {keyState === undefined ? (
        <div aria-busy="true">
          <progress
            className="dashboard-progress"
            aria-label="Loading browser key"
          />
          <SkeletonRow />
        </div>
      ) : null}
      {keyState?.ed25519 ? (
        <>
          <form
            className="dashboard-form"
            aria-label="Developer key sign-in"
            method="post"
            onSubmit={(event) => {
              event.preventDefault();
              const submitter = (event.nativeEvent as SubmitEvent).submitter;
              const action =
                submitter?.getAttribute("value") ??
                (keyState.hasKey ? "existing" : "create");
              void withKey(action === "create");
            }}
          >
            <div className="dashboard-field">
              <label htmlFor="gateway-tenant">Gateway tenant</label>
              <p id="tenant-help">
                Use the tenant configured by your gateway operator.
              </p>
              <Input
                id="gateway-tenant"
                name="tenant"
                autoComplete="username"
                enterKeyHint="go"
                value={tenant}
                onChange={(event) => {
                  setTenant(event.target.value);
                  if (event.target.validity.valid) setTenantInvalid(false);
                }}
                onBlur={(event) => {
                  setTenantInvalid(event.target.matches(":user-invalid"));
                }}
                onInvalid={() => setTenantInvalid(true)}
                error={tenantInvalid}
                aria-invalid={tenantInvalid ? "true" : "false"}
                disabled={busy !== undefined}
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                maxLength={95}
                pattern={"[a-z0-9._\\-]{1,95}"}
                required
                aria-describedby={
                  tenantInvalid ? "tenant-help tenant-error" : "tenant-help"
                }
              />
              {tenantInvalid ? (
                <p id="tenant-error" className="dashboard-field-error">
                  Enter the tenant configured by your gateway operator.
                </p>
              ) : null}
            </div>
            <p>The private developer key stays in this browser.</p>
            {keyState.publicKey ? (
              <div className="dashboard-public-key">
                <span>Public key</span>
                <code>{keyState.publicKey}</code>
                <Button
                  type="button"
                  variant="secondary"
                  disabled={busy !== undefined}
                  onClick={async () => {
                    try {
                      await navigator.clipboard.writeText(keyState.publicKey!);
                      setCopied(true);
                    } catch {
                      setMessage("The public key could not be copied.");
                    }
                  }}
                >
                  Copy public key
                </Button>
                <span role="status">{copied ? "Public key copied" : ""}</span>
              </div>
            ) : null}
            <Button
              type="submit"
              name="key-action"
              value="create"
              className="dashboard-key-create"
              loading={busy === "create"}
              disabled={busy !== undefined || keyState.hasKey}
            >
              Create a developer key in this browser and sign in
            </Button>
            <Button
              type="submit"
              name="key-action"
              value="existing"
              variant="secondary"
              loading={busy === "key"}
              disabled={busy !== undefined || !keyState.hasKey}
            >
              Sign in with the key this browser holds
            </Button>
          </form>
        </>
      ) : keyState ? (
        <p>
          This browser does not support Ed25519 keys; sign in with a session
          token.
        </p>
      ) : null}
      <form
        className="dashboard-form"
        aria-label="Session token sign-in"
        method="post"
        onSubmit={(event) => {
          event.preventDefault();
          const sessionToken = token;
          setToken("");
          setTokenInvalid(false);
          setBusy("token");
          setMessage("");
          void sessionRequest("/session", { token: sessionToken })
            .then(onSignedIn)
            .catch((cause) => setMessage(refusalWords(cause)))
            .finally(() => setBusy(undefined));
        }}
      >
        <div className="dashboard-field">
          <label htmlFor="session-token">Session token</label>
          <Input
            id="session-token"
            name="token"
            type="password"
            autoComplete="current-password"
            enterKeyHint="go"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            value={token}
            onChange={(event) => {
              setToken(event.target.value);
              if (event.target.validity.valid) setTokenInvalid(false);
            }}
            onBlur={(event) =>
              setTokenInvalid(event.target.matches(":user-invalid"))
            }
            onInvalid={() => setTokenInvalid(true)}
            error={tokenInvalid}
            aria-invalid={tokenInvalid ? "true" : "false"}
            aria-describedby={tokenInvalid ? "token-error" : undefined}
            disabled={busy !== undefined}
            maxLength={4096}
            required
          />
          {tokenInvalid ? (
            <p id="token-error" className="dashboard-field-error">
              Enter a valid session token.
            </p>
          ) : null}
        </div>
        <Button
          type="submit"
          variant="secondary"
          loading={busy === "token"}
          disabled={busy !== undefined}
        >
          Sign in with a session token
        </Button>
      </form>
      <p role="alert">{message}</p>
    </Card>
  );
}

function OverviewPanel({ value }: { value: Overview }) {
  return (
    <>
      <Card className="dashboard-card">
        <StatPair
          className="dashboard-usage"
          left={{ value: value.usage.requests_used, label: "Requests used" }}
          right={{
            value: value.usage.requests_remaining,
            label: "Requests remaining",
          }}
        />
      </Card>
      <div className="dashboard-stats">
        <Card className="dashboard-card">
          <Stat value={value.usage.live_keys} label="Live API keys" />
        </Card>
        <Card className="dashboard-card">
          <Stat value={value.endpoints.length} label="Webhook endpoints" />
        </Card>
        <Card className="dashboard-card">
          <Stat
            value={value.payments.filter((payment) => payment.settled).length}
            label="Verified test payments"
          />
        </Card>
      </div>
      <DashboardCard title="Recent protocol activity">
        {value.recent_requests.length === 0 ? (
          <EmptyState title="No recent protocol activity" />
        ) : (
          <List>
            {value.recent_requests.slice(0, 8).map((request) => (
              <ListItem
                key={`${request.at}-${request.operation_digest}`}
                title={request.outcome}
                subtitle={request.operation_digest}
                trailing={verificationBadge(request.verification)}
                trailingCaption={new Date(request.at * 1000).toLocaleString()}
              />
            ))}
          </List>
        )}
      </DashboardCard>
    </>
  );
}

function KeysPanel({ keys }: { keys: KeyView[] }) {
  return (
    <DashboardCard title="API keys and current quota windows">
      {keys.length === 0 ? (
        <EmptyState
          title="No API keys"
          description="Issue a principal-bound key through the gateway key management surface."
        />
      ) : (
        <List>
          {keys.map((key) => (
            <ListItem
              key={key.key_id}
              title={key.key_id}
              subtitle={`${key.used_in_window} of ${key.requests_per_window} requests used · ${key.window_seconds}s window`}
              trailing={
                <Badge
                  className="dashboard-badge"
                  data-tone={key.disabled ? "destructive" : "success"}
                  variant={key.disabled ? "destructive" : "success"}
                >
                  {key.disabled ? "revoked" : "active"}
                </Badge>
              }
              trailingCaption={`${key.remaining_in_window} remaining`}
            />
          ))}
        </List>
      )}
    </DashboardCard>
  );
}

function RequestsPanel({ requests }: { requests: RequestRecord[] }) {
  return (
    <DashboardCard title="Principal-isolated request log">
      {requests.length === 0 ? (
        <EmptyState title="No requests" />
      ) : (
        <List>
          {requests.map((request) => (
            <ListItem
              key={`${request.at}-${request.operation_digest}`}
              title={request.outcome}
              subtitle={request.operation_digest}
              trailing={verificationBadge(request.verification)}
              trailingCaption={new Date(request.at * 1000).toLocaleString()}
            />
          ))}
        </List>
      )}
    </DashboardCard>
  );
}

function WebhooksPanel({
  endpoints,
  deliveries,
}: {
  endpoints: Endpoint[];
  deliveries: Delivery[];
}) {
  return (
    <div className="dashboard-webhooks">
      <DashboardCard title="Webhook endpoints">
        {endpoints.length === 0 ? (
          <EmptyState title="No webhook endpoints" />
        ) : (
          <List>
            {endpoints.map((endpoint) => (
              <ListItem
                key={endpoint.endpoint}
                title={endpoint.url}
                subtitle={`${endpoint.pending} pending · ${endpoint.in_flight} in flight · ${endpoint.retrying} retrying`}
                trailing={
                  <Badge
                    className="dashboard-badge"
                    data-tone={endpoint.suspended ? "destructive" : "success"}
                    variant={endpoint.suspended ? "destructive" : "success"}
                  >
                    {endpoint.suspended ? "suspended" : "active"}
                  </Badge>
                }
                trailingCaption={`${endpoint.delivered_total} delivered / ${endpoint.dead_lettered_total} dead-lettered`}
              />
            ))}
          </List>
        )}
      </DashboardCard>
      <DashboardCard title="Delivery log">
        {deliveries.length === 0 ? (
          <EmptyState title="No deliveries" />
        ) : (
          <List>
            {deliveries.map((delivery) => (
              <ListItem
                key={delivery.delivery}
                title={delivery.event}
                subtitle={`${delivery.endpoint} · ${delivery.delivery}`}
                trailing={verificationBadge(delivery.verification)}
                trailingCaption={delivery.state.state}
              />
            ))}
          </List>
        )}
      </DashboardCard>
    </div>
  );
}

function PaymentsPanel({
  payments,
  receiptActivity,
  onReceipt,
}: {
  payments: Payment[];
  receiptActivity?: string;
  onReceipt: (activity: string) => Promise<void>;
}) {
  return (
    <DashboardCard title="Real test payments and verified receipts">
      {payments.length === 0 ? (
        <EmptyState
          title="No test payments"
          description="Payments appear only after the canonical payment source and independent receipt authority agree."
        />
      ) : (
        <List>
          {payments.map((payment, index) => (
            <ListItem
              key={payment.event}
              title={`${payment.amount ?? "—"} ${payment.asset ?? ""}`}
              subtitle={
                <>
                  <span id={`payment-event-${index}`}>{payment.event}</span>
                  {` · ${payment.facts.map((fact) => `${fact.name}=${fact.value} [${fact.verification}]`).join(" · ")} · receipt ${payment.receipt_digest ?? "not available"} · all facts ${payment.verification}`}
                </>
              }
              trailing={
                <span className="dashboard-receipt-actions">
                  {verificationBadge(payment.settlement_verification)}
                  {payment.settled
                    ? payment.facts
                        .filter(
                          (fact) =>
                            fact.name === "activity_id" &&
                            fact.verification !== "unverified" &&
                            fact.receipt_digest === payment.receipt_digest,
                        )
                        .map((fact) => (
                          <Button
                            key={fact.value}
                            type="button"
                            variant="secondary"
                            aria-describedby={`payment-event-${index}`}
                            loading={receiptActivity === fact.value}
                            disabled={receiptActivity !== undefined}
                            onClick={() => void onReceipt(fact.value)}
                          >
                            View receipt
                          </Button>
                        ))
                    : null}
                </span>
              }
              trailingCaption={
                payment.settled
                  ? "settlement receipt verified"
                  : "settlement not verified"
              }
            />
          ))}
        </List>
      )}
    </DashboardCard>
  );
}
