import React, {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { formatUnits, type Address, type Hex } from "viem";
import { Button, Card, Flex, Input, Text } from "@paxeer-network/ui";
import {
  AgentOwnerClient,
  browserAgentKeySupported,
  prepareAgentClaim,
  parseAgentClaim,
  checkedAgentAmount,
  type AgentClaimRequest,
  type AgentKeyDisclosure,
  type OwnedAgent,
  type AgentDetail,
  type AgentActivity,
} from "@paxeer-network/paxeer-backend/core/wallet/embedded/sdk/agents";
import {
  buildPerpsAgentPreset,
  savePerpsAgentPreset,
  PERPS_AGENT_FUNCTIONS,
  PERPS_AGENT_APPROVAL,
  type ReadAgentBalances,
  type AgentWalletBalances,
} from "@paxeer-network/paxeer-backend/core/wallet/embedded/agentPreset";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";

type Load<T> =
  | { status: "loading"; message?: never }
  | { status: "error"; message: string }
  | { status: "ready"; value: T; refreshing?: boolean; message?: string };
function refreshingLoad<T>(current: Load<T>): Load<T> {
  return current.status === "ready"
    ? { status: "ready", value: current.value, refreshing: true }
    : { status: "loading" };
}
function failedLoad<T>(current: Load<T>, error: unknown): Load<T> {
  const message = errorMessage(error);
  return current.status === "ready"
    ? { status: "ready", value: current.value, message }
    : { status: "error", message };
}
function errorMessage(error: unknown): string {
  return error instanceof Error
    ? error.message
    : "Unable to load agent wallets";
}
const stack = "oui-flex oui-flex-col oui-gap-4 oui-min-w-0";
const textarea =
  "oui-w-full oui-rounded oui-bg-base-6 oui-p-3 oui-text-xs oui-text-base-contrast focus-visible:oui-outline-primary-light";

export function AgentsAccess(props: {
  state: "loading" | "unconfigured" | "external" | "signed_out";
  children?: ReactNode;
}) {
  return (
    <Card title="Agents" className="oui-bg-base-9 oui-font-semibold">
      <div className={stack}>
        <Text as="p" size="sm" intensity={54}>
          An agent wallet is a separate wallet for an automated trading agent.
          You fund it, set its limits, and can freeze it or sweep its funds back
          to your embedded wallet.
        </Text>
        {props.state === "loading" && (
          <p role="status">Loading embedded wallet…</p>
        )}
        {props.state === "unconfigured" && (
          <p role="status">
            Agent wallets are unavailable because the wallet gateway is not
            configured.
          </p>
        )}
        {props.state === "external" && (
          <p>
            Agents need the embedded wallet. Sign in to your embedded wallet and
            connect it to manage agents.
          </p>
        )}
        {props.state === "signed_out" && (
          <p>Sign in to the embedded wallet to manage your agents.</p>
        )}
        {props.children}
      </div>
    </Card>
  );
}

export interface AgentsWorkspaceProps {
  client: AgentOwnerClient;
  ownerUserId: string;
  ownerAddress: Address;
  readBalances: ReadAgentBalances;
  waitForReceipt: (hash: Hex) => Promise<"success" | "reverted">;
  explorerUrl: string;
}
export function AgentsWorkspace(props: AgentsWorkspaceProps) {
  const { client, ownerUserId, ownerAddress, readBalances } = props;
  const [inventory, setInventory] = useState<
    Load<{ agent: OwnedAgent; balances: AgentWalletBalances | null }[]>
  >({ status: "loading" });
  const [ownerBalance, setOwnerBalance] = useState<Load<AgentWalletBalances>>({
    status: "loading",
  });
  const [selected, setSelected] = useState("");
  const [revision, setRevision] = useState(0);
  const [name, setName] = useState("");
  const [keySupported, setKeySupported] = useState<boolean | null>(null);
  const [key, setKey] = useState<AgentKeyDisclosure | null>(null);
  const [claimDid, setClaimDid] = useState("");
  const [claimRequest, setClaimRequest] = useState<AgentClaimRequest | null>(
    null,
  );
  const [claimText, setClaimText] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const alive = useRef(true);
  const running = useRef(false);
  const loadGeneration = useRef(0);
  const refresh = useCallback(async () => {
    const generation = ++loadGeneration.current;
    setInventory(refreshingLoad);
    setOwnerBalance(refreshingLoad);
    await Promise.all([
      (async () => {
        try {
          const { agents } = await client.list();
          const values = await Promise.all(
            agents.map(async (agent) => ({
              agent,
              balances: agent.address
                ? await readBalances(agent.address)
                : null,
            })),
          );
          if (alive.current && generation === loadGeneration.current) {
            setInventory({ status: "ready", value: values });
            setSelected((current) =>
              agents.some((agent) => agent.did === current)
                ? current
                : (agents[0]?.did ?? ""),
            );
          }
        } catch (error) {
          if (alive.current && generation === loadGeneration.current)
            setInventory((current) => failedLoad(current, error));
        }
      })(),
      (async () => {
        try {
          const value = await readBalances(ownerAddress);
          if (alive.current && generation === loadGeneration.current)
            setOwnerBalance({ status: "ready", value });
        } catch (error) {
          if (alive.current && generation === loadGeneration.current)
            setOwnerBalance((current) => failedLoad(current, error));
        }
      })(),
    ]);
    if (alive.current && generation === loadGeneration.current)
      setRevision((value) => value + 1);
  }, [client, ownerAddress, readBalances]);
  useEffect(() => {
    alive.current = true;
    void refresh();
    void browserAgentKeySupported().then((supported) => {
      if (alive.current) setKeySupported(supported);
    });
    const hide = () => setKey(null);
    window.addEventListener("pagehide", hide);
    return () => {
      alive.current = false;
      loadGeneration.current++;
      window.removeEventListener("pagehide", hide);
    };
  }, [refresh]);
  const run = async (
    label: string,
    action: () => Promise<void>,
    reload = true,
  ) => {
    if (running.current) return;
    running.current = true;
    setBusy(label);
    setActionError(null);
    setNotice(null);
    try {
      await action();
      if (alive.current && reload) await refresh();
    } catch (error) {
      if (alive.current) setActionError(errorMessage(error));
    } finally {
      running.current = false;
      if (alive.current) setBusy(null);
    }
  };
  const retry = () => void run("Refreshing", refresh, false);
  return (
    <Card
      title="Agents"
      id="portfolio-agents"
      className="oui-bg-base-9 oui-font-semibold"
    >
      <div className={stack}>
        <Text as="p" size="sm" intensity={54}>
          Create a separate wallet for an automated trading agent. Fund it from
          your embedded wallet, set limits, freeze trading, and sweep available
          funds back.
        </Text>
        <Text as="p" size="xs" className="oui-break-all">
          Your embedded wallet: {ownerAddress}
        </Text>
        <BalanceView
          value={ownerBalance}
          label="Embedded wallet"
          retry={retry}
        />
        {busy && <p role="status">{busy}…</p>}
        {notice && <p role="status">{notice}</p>}
        {actionError && (
          <div role="alert">
            <p>{actionError}</p>
            <Button
              size="md"
              variant="outlined"
              disabled={!!busy}
              onClick={retry}
            >
              Refresh and retry
            </Button>
          </div>
        )}
        {key && (
          <section aria-label="Agent key" className={stack}>
            <Text as="p" color="warning" size="sm">
              Save this agent key now. It controls the agent and is shown only
              once. It cannot be recovered after you dismiss this view or leave
              the page. Keep it private.
            </Text>
            <label className={stack}>
              Agent credentials (32-byte Ed25519 seed in hex)
              <textarea
                className={textarea}
                readOnly
                rows={5}
                value={JSON.stringify(key, null, 2)}
                spellCheck={false}
                autoComplete="off"
              />
            </label>
            <Flex gap={2} wrap="wrap">
              <Button
                size="md"
                onClick={() =>
                  void run(
                    "Copying key",
                    async () => {
                      await navigator.clipboard.writeText(JSON.stringify(key));
                      setNotice("Agent key copied.");
                    },
                    false,
                  )
                }
              >
                Copy agent key
              </Button>
              <Button size="md" variant="outlined" onClick={() => setKey(null)}>
                I saved the key
              </Button>
            </Flex>
          </section>
        )}
        <section aria-labelledby="create-agent-heading" className={stack}>
          <h2 id="create-agent-heading">Create an agent</h2>
          <Text as="p" size="xs" intensity={54}>
            A new agent stays frozen after setup. Save its limits before
            unfreezing it.
          </Text>
          {keySupported === null && (
            <p role="status">Checking browser key support…</p>
          )}
          {keySupported === false && (
            <p role="status">
              This browser does not support Web Crypto Ed25519. Use a browser
              with Ed25519 support or claim an agent that holds its own key.
            </p>
          )}
          <label>
            Agent name
            <Input
              aria-label="Agent name"
              value={name}
              onValueChange={setName}
              maxLength={128}
              autoComplete="off"
            />
          </label>
          <Button
            size="md"
            disabled={
              !!busy ||
              key !== null ||
              keySupported !== true ||
              !/^[A-Za-z0-9_-]{1,128}$/.test(name)
            }
            onClick={() =>
              void run("Creating agent", async () => {
                const result = await client.create(
                  name,
                  ownerUserId,
                  (disclosure) => {
                    if (!alive.current) throw new Error("Agent page closed");
                    setKey(disclosure);
                  },
                );
                if (alive.current) {
                  setSelected(result.did);
                  setNotice(
                    "Agent created and frozen. Save limits before unfreezing.",
                  );
                  setName("");
                }
              })
            }
          >
            Create agent
          </Button>
        </section>
        <section aria-labelledby="claim-agent-heading" className={stack}>
          <h2 id="claim-agent-heading">
            Claim an agent that holds its own key
          </h2>
          <Text as="p" size="xs" intensity={54}>
            The agent must already be registered with the gateway. Only its
            signature is pasted here; keep its private key with the agent.
          </Text>
          <label>
            Agent DID
            <Input
              aria-label="Agent DID"
              value={claimDid}
              maxLength={256}
              onValueChange={(value) => {
                setClaimDid(value);
                setClaimRequest(null);
                setClaimText("");
              }}
            />
          </label>
          <Button
            size="md"
            variant="outlined"
            disabled={!!busy || !claimDid}
            onClick={() =>
              void run(
                "Preparing claim",
                async () => {
                  setClaimRequest(
                    await prepareAgentClaim(claimDid, ownerUserId),
                  );
                  setClaimText("");
                },
                false,
              )
            }
          >
            Prepare claim request
          </Button>
          {claimRequest && (
            <>
              <Text as="p" size="xs">
                Ask the agent to Ed25519-sign the 32 digest bytes below, decoded
                from hex. The digest is SHA-256 of PXW:AGENT-CLAIM:v1, the UTF-8
                DID and owner ID each prefixed by a 4-byte big-endian byte
                length, then the expiry as an 8-byte big-endian Unix timestamp.
                This request expires at{" "}
                {new Date(Number(claimRequest.expires) * 1000).toISOString()}.
              </Text>
              <pre
                className="oui-whitespace-pre-wrap oui-break-all oui-text-xs"
                aria-label="Claim request"
              >
                {JSON.stringify(claimRequest, null, 2)}
              </pre>
              <label className={stack}>
                Signed claim JSON (expires and signature)
                <textarea
                  aria-label="Signed claim JSON"
                  className={textarea}
                  rows={3}
                  maxLength={2048}
                  value={claimText}
                  onChange={(event) => setClaimText(event.target.value)}
                  spellCheck={false}
                  autoComplete="off"
                />
              </label>
              <Button
                size="md"
                disabled={!!busy || !claimText}
                onClick={() =>
                  void run("Claiming agent", async () => {
                    await client.claim(
                      claimRequest.did,
                      parseAgentClaim(claimText, claimRequest),
                    );
                    if (alive.current) {
                      setSelected(claimRequest.did);
                      setClaimRequest(null);
                      setClaimText("");
                      setNotice(
                        "Agent claimed. Review its limits and frozen state.",
                      );
                    }
                  })
                }
              >
                Claim agent
              </Button>
            </>
          )}
        </section>
        <section aria-labelledby="owned-agent-heading" className={stack}>
          <Flex justify="between" gap={2}>
            <h2 id="owned-agent-heading">Your agents</h2>
            <Button
              size="md"
              variant="outlined"
              disabled={!!busy}
              onClick={retry}
            >
              Refresh agents
            </Button>
          </Flex>
          {inventory.status === "loading" && (
            <p role="status">Loading agents and balances…</p>
          )}
          {inventory.status === "ready" && inventory.refreshing && (
            <p role="status">Refreshing agents and balances…</p>
          )}
          {inventory.message && (
            <div role="alert">
              {inventory.message}
              <Button
                size="md"
                variant="outlined"
                disabled={!!busy}
                onClick={retry}
              >
                Retry agents
              </Button>
            </div>
          )}
          {inventory.status === "ready" && inventory.value.length === 0 && (
            <p>No agents yet. Create an agent or claim one above.</p>
          )}
          {inventory.status === "ready" &&
            inventory.value.map(({ agent, balances }) => (
              <div
                key={agent.did}
                className="oui-rounded oui-bg-base-6 oui-p-3 oui-text-xs"
              >
                <div className={stack}>
                  <Text as="p" className="oui-break-all">
                    {agent.label}: {agent.did}
                  </Text>
                  <Text as="p" className="oui-break-all">
                    Wallet: {agent.address ?? "Wallet not provisioned"}
                  </Text>
                  <Text as="p">
                    Mode: {agent.mode}. {agent.is_frozen ? "Frozen" : "Active"}.
                  </Text>
                  {balances && (
                    <Text as="p">
                      {formatUnits(balances.native, 18)} PAX ·{" "}
                      {formatUnits(
                        balances.collateral,
                        COLLATERAL_TOKENS.usdl.decimals,
                      )}{" "}
                      USDL
                    </Text>
                  )}
                  <Button
                    size="md"
                    variant={selected === agent.did ? "contained" : "outlined"}
                    disabled={!!busy}
                    aria-pressed={selected === agent.did}
                    onClick={() => setSelected(agent.did)}
                  >
                    Manage {agent.label}
                  </Button>
                </div>
              </div>
            ))}
        </section>
        {selected && (
          <AgentControls
            key={selected}
            {...props}
            did={selected}
            revision={revision}
            busy={!!busy}
            run={run}
          />
        )}
      </div>
    </Card>
  );
}

function BalanceView({
  value,
  label,
  retry,
}: {
  value: Load<AgentWalletBalances>;
  label: string;
  retry: () => void;
}) {
  if (value.status === "loading")
    return <p role="status">Loading {label.toLowerCase()} balances…</p>;
  return (
    <>
      {value.message && (
        <div role="alert">
          {label}: {value.message}
          <Button size="md" variant="outlined" onClick={retry}>
            Retry balances
          </Button>
        </div>
      )}
      {value.status === "ready" && (
        <>
          {value.refreshing && (
            <p role="status">Refreshing {label.toLowerCase()} balances…</p>
          )}
          <Text as="p" size="xs">
            {label} balance: {formatUnits(value.value.native, 18)} PAX ·{" "}
            {formatUnits(
              value.value.collateral,
              COLLATERAL_TOKENS.usdl.decimals,
            )}{" "}
            USDL
          </Text>
        </>
      )}
    </>
  );
}

type RunAction = (
  label: string,
  action: () => Promise<void>,
  reload?: boolean,
) => Promise<void>;
function AgentControls(
  props: AgentsWorkspaceProps & {
    did: string;
    revision: number;
    busy: boolean;
    run: RunAction;
  },
) {
  const { client, did, revision, run, readBalances, ownerAddress } = props;
  const [detail, setDetail] = useState<Load<AgentDetail>>({
    status: "loading",
  });
  const [activity, setActivity] = useState<Load<AgentActivity[]>>({
    status: "loading",
  });
  const [tick, setTick] = useState(0);
  const [asset, setAsset] = useState<"collateral" | "native">("collateral");
  const [amount, setAmount] = useState("");
  const [txCap, setTxCap] = useState("");
  const [dayCap, setDayCap] = useState("");
  const [days, setDays] = useState("30");
  const [transfer, setTransfer] = useState<{
    hash: Hex;
    state: string;
    kind: string;
  } | null>(null);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    let active = true;
    setDetail(refreshingLoad);
    setActivity(refreshingLoad);
    void client
      .detail(did)
      .then((value) => {
        if (active) setDetail({ status: "ready", value });
      })
      .catch((error) => {
        if (active) setDetail((current) => failedLoad(current, error));
      });
    void client
      .activity(did)
      .then(({ events }) => {
        if (active) setActivity({ status: "ready", value: events });
      })
      .catch((error) => {
        if (active) setActivity((current) => failedLoad(current, error));
      });
    return () => {
      active = false;
      alive.current = false;
    };
  }, [client, did, revision, tick]);
  const busy = props.busy || (detail.status === "ready" && !!detail.refreshing);
  const retry = () => setTick((value) => value + 1);
  let preset: ReturnType<typeof buildPerpsAgentPreset> | null = null;
  let limitsError = "Enter both caps to review the limits.";
  try {
    if (!/^\d+$/.test(days))
      throw new Error("Enter a whole number of budget days");
    preset = buildPerpsAgentPreset(
      ownerAddress,
      txCap,
      dayCap,
      Number(days) * 86400,
    );
  } catch (error) {
    limitsError = errorMessage(error);
  }
  const send = async (kind: "Fund" | "Sweep") => {
    if (detail.status !== "ready" || !detail.value.wallet)
      throw new Error("The agent wallet is not provisioned");
    const source = kind === "Fund" ? ownerAddress : detail.value.wallet.address;
    const balances = await readBalances(source);
    const native = asset === "native";
    const raw = checkedAgentAmount(
      amount,
      native ? 18 : COLLATERAL_TOKENS.usdl.decimals,
      native ? balances.native : balances.collateral,
    );
    if (balances.native === 0n || (native && BigInt(raw) === balances.native))
      throw new Error("Leave PAX in the source wallet to pay transaction gas");
    const request = {
      token: native ? null : COLLATERAL_TOKENS.usdl.address,
      amount: raw,
    };
    const result =
      kind === "Fund"
        ? await client.fund(did, request)
        : await client.sweep(did, request);
    if (alive.current)
      setTransfer({ hash: result.tx_hash, state: "Submitted", kind });
    const receipt = await props.waitForReceipt(result.tx_hash);
    if (alive.current) {
      setTransfer({
        hash: result.tx_hash,
        state: receipt === "success" ? "Confirmed" : "Reverted",
        kind,
      });
      setAmount("");
    }
  };
  return (
    <section aria-label="Manage agent" className={stack}>
      {detail.status === "loading" && (
        <p role="status">Loading agent settings…</p>
      )}
      {detail.status === "ready" && detail.refreshing && (
        <p role="status">Refreshing agent settings…</p>
      )}
      {detail.message && (
        <div role="alert">
          {detail.message}
          <Button size="md" variant="outlined" disabled={busy} onClick={retry}>
            Retry settings
          </Button>
        </div>
      )}
      {detail.status === "ready" && (
        <>
          <h2>Manage {detail.value.label}</h2>
          <Text as="p" size="xs">
            Current mode: {detail.value.policy.mode}.{" "}
            {detail.value.is_frozen ? "Frozen" : "Active"}. Native value caps:{" "}
            {formatUnits(BigInt(detail.value.policy.max_tx_value_wei), 18)} PAX
            per transaction and{" "}
            {formatUnits(BigInt(detail.value.policy.max_daily_value_wei), 18)}{" "}
            PAX per day.
          </Text>
          <Flex gap={2} wrap="wrap">
            <Button
              size="md"
              variant="outlined"
              disabled={busy}
              onClick={() =>
                void run(
                  detail.value.is_frozen
                    ? "Unfreezing agent"
                    : "Freezing agent",
                  async () => {
                    await client.freeze(did, !detail.value.is_frozen);
                  },
                )
              }
            >
              {detail.value.is_frozen ? "Unfreeze agent" : "Freeze agent"}
            </Button>
          </Flex>
          <section aria-labelledby="agent-fund-heading" className={stack}>
            <h3 id="agent-fund-heading">
              Fund or sweep available wallet funds
            </h3>
            <Text as="p" size="xs" intensity={54}>
              Funding moves funds from your embedded wallet. Sweeping returns
              the selected amount to that wallet. Close positions and withdraw
              deposited collateral before sweeping those funds. Keep PAX for
              gas.
            </Text>
            <Flex gap={2} wrap="wrap">
              <Button
                size="md"
                variant={asset === "collateral" ? "contained" : "outlined"}
                aria-pressed={asset === "collateral"}
                disabled={busy}
                onClick={() => {
                  setAsset("collateral");
                  setAmount("");
                }}
              >
                USDL
              </Button>
              <Button
                size="md"
                variant={asset === "native" ? "contained" : "outlined"}
                aria-pressed={asset === "native"}
                disabled={busy}
                onClick={() => {
                  setAsset("native");
                  setAmount("");
                }}
              >
                PAX
              </Button>
            </Flex>
            <label>
              Amount ({asset === "native" ? "PAX" : "USDL"})
              <Input
                aria-label="Transfer amount"
                value={amount}
                onValueChange={setAmount}
                inputMode="decimal"
                maxLength={80}
                disabled={busy}
              />
            </label>
            <Flex gap={2} wrap="wrap">
              <Button
                size="md"
                disabled={busy || !amount || !detail.value.wallet}
                onClick={() => void run("Funding agent", () => send("Fund"))}
              >
                Fund agent
              </Button>
              <Button
                size="md"
                variant="outlined"
                disabled={busy || !amount || !detail.value.wallet}
                onClick={() =>
                  void run("Sweeping to embedded wallet", () => send("Sweep"))
                }
              >
                Sweep to my wallet
              </Button>
            </Flex>
            {transfer && (
              <p role="status" className="oui-break-all">
                {transfer.kind}: {transfer.state}.{" "}
                <a
                  href={`${props.explorerUrl.replace(/\/$/, "")}/tx/${transfer.hash}`}
                  target="_blank"
                  rel="noreferrer"
                  className="oui-text-primary-light"
                >
                  {transfer.hash}
                </a>
              </p>
            )}
          </section>
          <section aria-labelledby="agent-limits-heading" className={stack}>
            <h3 id="agent-limits-heading">Trading limits</h3>
            <label>
              Per-transaction native value cap (PAX)
              <Input
                aria-label="Per-transaction cap"
                value={txCap}
                onValueChange={setTxCap}
                maxLength={80}
                inputMode="decimal"
                disabled={busy}
              />
            </label>
            <label>
              Daily native value cap (PAX)
              <Input
                aria-label="Daily cap"
                value={dayCap}
                onValueChange={setDayCap}
                maxLength={80}
                inputMode="decimal"
                disabled={busy}
              />
            </label>
            <label>
              Budget duration (days)
              <Input
                aria-label="Budget duration"
                value={days}
                onValueChange={setDays}
                inputMode="numeric"
                maxLength={3}
                disabled={busy}
              />
            </label>
            {preset ? (
              <div aria-label="Limits preview" className={stack}>
                {preset.sentences.map((sentence) => (
                  <Text as="p" size="xs" key={sentence}>
                    {sentence}
                  </Text>
                ))}
              </div>
            ) : (
              <p role="status">{limitsError}</p>
            )}
            <details>
              <summary>Contracts and functions in the preset</summary>
              <ul className="oui-text-xs oui-break-all">
                {PERPS_AGENT_FUNCTIONS.map((call) => (
                  <li key={call.selector}>
                    {call.name}: {call.contract} · {call.selector}
                  </li>
                ))}
                <li>
                  approve: {PERPS_AGENT_APPROVAL.contract} ·{" "}
                  {PERPS_AGENT_APPROVAL.selector}. Spender:{" "}
                  {PERPS_AGENT_APPROVAL.spender}
                </li>
              </ul>
            </details>
            <Button
              size="md"
              disabled={busy || !preset}
              onClick={() => {
                const reviewed = preset;
                if (reviewed)
                  void run("Saving limits; agent stays frozen", async () => {
                    await savePerpsAgentPreset(client, did, reviewed);
                  });
              }}
            >
              Save limits and keep frozen
            </Button>
            {detail.value.budgets.length === 0 ? (
              <Text as="p" size="xs" intensity={54}>
                No active budgets.
              </Text>
            ) : (
              <ul className="oui-text-xs">
                {detail.value.budgets.map((budget) => (
                  <li key={budget.id}>
                    Budget remaining: {budget.remaining_wei} base units of{" "}
                    {budget.token ?? "PAX"}. Expires{" "}
                    {new Date(budget.expires_at).toLocaleString()}.
                  </li>
                ))}
              </ul>
            )}
          </section>
        </>
      )}
      <section aria-labelledby="agent-activity-heading" className={stack}>
        <h3 id="agent-activity-heading">Activity</h3>
        {activity.status === "loading" && (
          <p role="status">Loading activity…</p>
        )}
        {activity.status === "ready" && activity.refreshing && (
          <p role="status">Refreshing activity…</p>
        )}
        {activity.message && (
          <div role="alert">
            {activity.message}
            <Button
              size="md"
              variant="outlined"
              disabled={busy}
              onClick={retry}
            >
              Retry activity
            </Button>
          </div>
        )}
        {activity.status === "ready" &&
          (activity.value.length === 0 ? (
            <p>No activity yet.</p>
          ) : (
            <div className="oui-overflow-x-auto">
              <table className="oui-w-full oui-text-xs">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Kind</th>
                    <th>Native amount</th>
                    <th>Result</th>
                  </tr>
                </thead>
                <tbody>
                  {activity.value.map((event, index) => (
                    <tr key={`${event.created_at}:${index}`}>
                      <td>
                        <time dateTime={event.created_at}>
                          {new Date(event.created_at).toLocaleString()}
                        </time>
                      </td>
                      <td>{event.kind}</td>
                      <td>{formatUnits(BigInt(event.value_wei), 18)} PAX</td>
                      <td>
                        {event.tx_hash ? (
                          <a
                            className="oui-text-primary-light oui-break-all"
                            href={`${props.explorerUrl.replace(/\/$/, "")}/tx/${event.tx_hash}`}
                            target="_blank"
                            rel="noreferrer"
                          >
                            Broadcast: {event.tx_hash}
                          </a>
                        ) : (
                          "Signed"
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
      </section>
    </section>
  );
}
