import assert from "node:assert/strict";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { test, type TestContext } from "node:test";
import {
  createPublicClient,
  createWalletClient,
  encodeFunctionData,
  erc20Abi,
  formatUnits,
  getAddress,
  http,
  isHex,
  keccak256,
  parseEventLogs,
  type Address,
  type Hex,
  type TransactionReceipt,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { PerpsPrecompileABI } from "@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile";
import * as calls from "@paxeer-network/paxeer-backend/core/perps/calls";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";
import { PERPS_AGENT_FUNCTIONS } from "@paxeer-network/paxeer-backend/core/wallet/embedded/agentPreset";
import { positionFromTuple } from "@paxeer-network/paxeer-backend/core/perps/positionWrites";
import { buildClaim } from "../../../sdk/wallet/src/agent.ts";
import {
  ADDR_PRECOMPILE,
  addrAbi,
  mainAccountId,
} from "../../../services/wallet/gateway/src/provision/protocol.ts";
import type { WalletInventoryKey } from "../../../services/wallet/gateway/src/agent/authority.ts";
import { startLocalWalletGateway } from "../../../services/wallet/gateway/test/support/local-gateway.ts";
import type { WireTransaction } from "../packages/agent-kit/src/gateway.ts";

const kitRequire = createRequire(
  new URL("../packages/agent-kit/package.json", import.meta.url),
);
const { ed25519 } = kitRequire(
  "@noble/curves/ed25519.js",
) as typeof import("@noble/curves/ed25519.js");
const {
  AgentGatewayClient,
  PerpsDataClient,
  createAgentTools,
  buildTradingPreset,
  AgentKitError,
} = kitRequire(
  "./src/index.ts",
) as typeof import("../packages/agent-kit/src/index.ts");
const WAD = 10n ** 18n;
const UNIT = 10n ** 6n;
const USDL = COLLATERAL_TOKENS.usdl.address;
const PERPS = calls.buildDeposit(1n).address;
const PERPS_CUSTODY = getAddress(
  `0x${createHash("sha256").update("perps").digest("hex").slice(0, 40)}`,
);

function required(name: string): string {
  const value = process.env[name];
  assert.ok(value, `${name} is required`);
  return value;
}

function fixtureKey(name: string): Hex {
  const value = required(name);
  assert.ok(isHex(value) && value.length === 66, `${name} must be 32 bytes`);
  return value;
}

function loopback(raw: string): string {
  const url = new URL(raw);
  assert.equal(url.protocol, "http:");
  assert.ok(["localhost", "127.0.0.1", "[::1]"].includes(url.hostname));
  assert.equal(url.username, "");
  assert.equal(url.password, "");
  return url.href;
}

function json(value: unknown): string {
  return JSON.stringify(value, (_, item: unknown) =>
    typeof item === "bigint" ? item.toString() : item,
  );
}

function record(value: unknown): Record<string, unknown> {
  assert.ok(
    value !== null && typeof value === "object" && !Array.isArray(value),
    json(value),
  );
  return value as Record<string, unknown>;
}

function array(value: unknown): unknown[] {
  assert.ok(Array.isArray(value), json(value));
  return value;
}

function hash(value: unknown): Hex {
  assert.ok(
    typeof value === "string" && /^0x[0-9a-fA-F]{64}$/.test(value),
    json(value),
  );
  return value as Hex;
}

function address(value: unknown): Address {
  assert.equal(typeof value, "string");
  return getAddress(value as string);
}

function syntheticUser(did: string): string {
  const bytes = createHash("sha256")
    .update(`paxeer-agent-wallet:${did}`)
    .digest()
    .subarray(0, 16);
  bytes[6] = (bytes[6]! & 15) | 128;
  bytes[8] = (bytes[8]! & 63) | 128;
  const hex = bytes.toString("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

async function localAgentJourney(t: TestContext) {
  const rpcUrl = loopback(required("LOCAL_NODE_RPC"));
  const dataUrl = loopback(required("NEXT_PUBLIC_PERPS_GATEWAY_URL"));
  const rpc = createPublicClient({
    transport: http(rpcUrl, { retryCount: 0 }),
  });
  assert.equal(await rpc.getChainId(), 125);
  const funded = privateKeyToAccount(fixtureKey("LOCAL_NODE_KEY"));
  const maker = privateKeyToAccount(fixtureKey("LOCAL_NODE_MAKER_KEY"));
  assert.equal(
    funded.address.toLowerCase(),
    required("LOCAL_NODE_ADDRESS").toLowerCase(),
  );
  assert.equal(
    maker.address.toLowerCase(),
    required("LOCAL_NODE_MAKER_ADDRESS").toLowerCase(),
  );
  assert.equal(
    USDL.toLowerCase(),
    required("LOCAL_NODE_COLLATERAL").toLowerCase(),
  );
  assert.notEqual(maker.address, funded.address);
  const fundedWallet = createWalletClient({
    account: funded,
    transport: http(rpcUrl),
  });
  const makerWallet = createWalletClient({
    account: maker,
    transport: http(rpcUrl),
  });
  const venue = new PerpsDataClient({ nodeUrl: rpcUrl, dataUrl });
  const fixture = await startLocalWalletGateway(rpcUrl, {
    tokenCaps: [
      { address: USDL, perTransaction: 1_000n * UNIT, daily: 10_000n * UNIT },
    ],
    selectors: {
      [PERPS]: PERPS_AGENT_FUNCTIONS.map((call) => call.selector),
    },
    trustedSpenders: [PERPS],
  });
  t.after(() => fixture.stop());
  const ownerId = randomUUID();
  const seed = randomBytes(32);
  const publicKey = Buffer.from(ed25519.getPublicKey(seed)).toString("hex");
  let readAuthorization: string | null = null;
  const agent = new AgentGatewayClient({
    url: fixture.url,
    privateKey: seed,
    label: `journey-${randomBytes(6).toString("hex")}`,
    fetch: async (input, init) => {
      const authorization = new Headers(init?.headers).get("authorization");
      if (authorization) readAuthorization = authorization;
      return fetch(input, init);
    },
  });
  const kit = createAgentTools(
    agent,
    new PerpsDataClient({ nodeUrl: fixture.rpc.url, dataUrl }),
  );
  const ownerRoute = `/v1/agents/${encodeURIComponent(agent.did)}`;
  const inventory: WalletInventoryKey[] = [];
  const receipts = new Map<Hex, TransactionReceipt>();
  const agentTransactions = new Set<Hex>();
  let ownerAddress: Address;
  let agentAddress: Address;
  let positionId: bigint;
  let orderId: bigint;
  let feeTokens = 0n;
  let grossPnl = 0n;
  let fundingPaid = 0n;
  let settlementDelta = 0n;
  let depositActionId: string;
  let tokenRuleId: string;
  const marketId = 11n;
  const tokenCap = 500n * UNIT;
  const depositAmount = 400n * UNIT;
  const fundedTokens = 1_000n * UNIT;
  const reports: unknown[] = [];

  function report(
    step: string,
    tool: string,
    route: string,
    reads: unknown,
    receipt?: TransactionReceipt,
  ) {
    const entry = {
      step,
      tool,
      route,
      transaction: receipt?.transactionHash ?? null,
      gas: receipt?.gasUsed ?? 0n,
      block: receipt?.blockNumber ?? null,
      reads,
    };
    reports.push(entry);
    t.diagnostic(json(entry));
  }
  t.after(() => t.diagnostic(json({ journey: reports })));

  async function request(
    method: string,
    route: string,
    body?: unknown,
    admin = false,
  ) {
    const token = admin
      ? fixture.adminToken
      : await fixture.identity.mint(ownerId);
    const response = await fetch(`${fixture.url}${route}`, {
      method,
      headers: {
        authorization: `Bearer ${token}`,
        ...(body === undefined ? {} : { "content-type": "application/json" }),
      },
      ...(body === undefined ? {} : { body: json(body) }),
      signal: AbortSignal.timeout(900_000),
    });
    assert.equal(response.headers.get("x-served-by"), "paxeer-wallet-gateway");
    return { status: response.status, body: record(await response.json()) };
  }

  async function owner(
    method: string,
    suffix: string,
    body?: unknown,
    status = 200,
  ) {
    const response = await request(method, `${ownerRoute}${suffix}`, body);
    assert.equal(
      response.status,
      status,
      `${method} ${suffix}: ${json(response.body)}\n${fixture.logs.slice(-15).join("")}`,
    );
    assert.equal(response.body.error, undefined, json(response.body));
    return response.body;
  }

  async function publishInventory() {
    fixture.network.authority.approveInventory(inventory);
    const response = await request(
      "POST",
      "/v1/admin/custody-inventory",
      {
        token: readFileSync(fixture.network.authority.inventoryFile, "utf8"),
      },
      true,
    );
    assert.equal(response.status, 200, json(response.body));
  }

  async function admitKeys(keyOwner: string) {
    for (const entry of inventory.filter(
      (entry) => entry.owner === keyOwner && entry.public_key === "",
    )) {
      entry.public_key = await fixture.network.publicKey(entry.key_id);
      entry.operations = ["generate", "sign"];
    }
    await publishInventory();
  }

  async function tool(name: string, input: unknown = {}) {
    const result = await kit.callTool(name, input);
    assert.equal(
      result.ok,
      true,
      `${name}: ${json(result)}\n${fixture.logs.slice(-15).join("")}`,
    );
    assert.ok(result.ok);
    return result.data;
  }

  async function receipt(tx: Hex, agentTx = false) {
    const mined = await rpc.waitForTransactionReceipt({
      hash: tx,
      timeout: 120_000,
    });
    assert.equal(mined.status, "success", tx);
    assert.ok(mined.gasUsed > 0n);
    assert.equal(mined.transactionHash, tx);
    const transaction = await rpc.getTransaction({ hash: tx });
    assert.equal(transaction.chainId, 125);
    assert.equal(transaction.blockNumber, mined.blockNumber);
    receipts.set(tx, mined);
    if (agentTx) agentTransactions.add(tx);
    return mined;
  }

  async function gatewayReceipt(tx: Hex) {
    const mined = await receipt(tx, true);
    const response = record(await tool("transaction_receipt", { hash: tx }));
    assert.deepEqual(response, {
      status: "success",
      block_number: mined.blockNumber.toString(),
      gas_used: mined.gasUsed.toString(),
      tx_hash: tx,
    });
    return mined;
  }

  async function write(name: string, input: unknown) {
    const result = record(await tool(name, input));
    return gatewayReceipt(hash(result.tx_hash));
  }

  function events(mined: TransactionReceipt) {
    return parseEventLogs({
      abi: PerpsPrecompileABI,
      logs: mined.logs,
      strict: true,
    });
  }

  function event<N extends ReturnType<typeof events>[number]["eventName"]>(
    mined: TransactionReceipt,
    name: N,
  ) {
    const found = events(mined).filter((entry) => entry.eventName === name);
    assert.equal(found.length, 1, `${name} in ${mined.transactionHash}`);
    const matched = found[0]!;
    assert.equal(matched.eventName, name);
    return matched as Extract<
      ReturnType<typeof events>[number],
      { eventName: N }
    >;
  }

  async function chainAccount() {
    return rpc.readContract(calls.buildGetAccount(agentAddress));
  }

  async function tokenBalance(account = agentAddress) {
    return rpc.readContract({
      address: USDL,
      abi: erc20Abi,
      functionName: "balanceOf",
      args: [account],
    });
  }

  async function accountAgreement() {
    const direct = await chainAccount();
    const result = record(await tool("account"));
    assert.equal(address(result.address), agentAddress);
    assert.deepEqual(result.state, direct);
    assert.equal(result.collateral, direct.collateral);
    assert.equal(result.withdrawable, direct.withdrawable);
    return direct;
  }

  async function fixtureSend(
    wallet: typeof fundedWallet,
    to: Address,
    data: Hex = "0x",
    value = 0n,
  ) {
    const tx = await wallet.sendTransaction({
      chain: null,
      to,
      data,
      value,
      gas: 2_000_000n,
      gasPrice: 1_000_000_000_000n,
      nonce: await rpc.getTransactionCount({ address: wallet.account.address }),
    });
    return receipt(tx);
  }

  async function makerCall(
    call:
      | ReturnType<typeof calls.buildDeposit>
      | ReturnType<typeof calls.buildPlaceLimitOrder>
      | ReturnType<typeof calls.buildCancelOrder>,
  ) {
    return fixtureSend(makerWallet, call.address, encodeFunctionData(call));
  }

  async function liquidity(buy: boolean) {
    for (const id of await rpc.readContract(
      calls.buildGetUserOrderIds(maker.address),
    )) {
      const cancelled = await makerCall(calls.buildCancelOrder(id));
      assert.equal(event(cancelled, "OrderCancelled").args.orderId, id);
      report(
        "maker cancel",
        "fixture wallet",
        "eth_sendRawTransaction",
        { orderId: id, active: (await venue.order(id)).active },
        cancelled,
      );
    }
    const mark = await venue.mark(marketId);
    assert.equal(mark.markPrice, WAD);
    assert.equal(mark.fresh, true);
    const mined = await makerCall(
      calls.buildPlaceLimitOrder({
        marketId,
        isLong: buy,
        limitPrice: WAD,
        sizeUsd: 1_000n * WAD,
        leverage: WAD,
        collateralAmount: 1_000n * UNIT,
        timeInForce: 3,
        reduceOnly: false,
        cloid: `0x${randomBytes(16).toString("hex")}`,
      }),
    );
    const placed = event(mined, "OrderPlaced").args;
    assert.equal(placed.user.toLowerCase(), maker.address.toLowerCase());
    const resting = await venue.order(placed.orderId);
    assert.equal(resting.active, true);
    assert.equal(resting.isLong, buy);
    assert.equal(resting.sizeUsd, 1_000n * WAD);
    report(
      "maker resting liquidity",
      "fixture wallet",
      "eth_sendRawTransaction",
      { orderId: placed.orderId, ...resting },
      mined,
    );
    return placed.orderId;
  }

  async function noBroadcast() {
    return {
      sends: fixture.rpc.broadcasts.length,
      nonce: await rpc.getTransactionCount({ address: agentAddress }),
      native: await rpc.getBalance({ address: agentAddress }),
      token: await tokenBalance(),
      allowance: await rpc.readContract({
        address: USDL,
        abi: erc20Abi,
        functionName: "allowance",
        args: [agentAddress, PERPS],
      }),
      account: await chainAccount(),
      orders: await rpc.readContract(calls.buildGetUserOrderIds(agentAddress)),
      positions: await rpc.readContract(
        calls.buildGetUserPositionIds(agentAddress),
      ),
    };
  }

  async function refusal(
    name: string,
    execute: () => Promise<unknown>,
    code: string,
    status: number,
  ) {
    const before = await noBroadcast();
    await assert.rejects(execute, (error: unknown) => {
      assert.ok(error instanceof AgentKitError, String(error));
      assert.equal(error.code, code);
      assert.equal(record(error.details).status, status);
      report(name, "agent gateway client", "/v1/agent/send", {
        status,
        code,
        state: before,
      });
      return true;
    });
    assert.deepEqual(await noBroadcast(), before);
  }

  async function run(step: (context: TestContext) => Promise<void>) {
    let passed = false;
    await t.test(step.name, async (context) => {
      await step(context);
      passed = true;
    });
    assert.ok(passed, step.name);
  }

  async function selfFundedBindingWaitsForCommittedState() {
    for (const [key_id, curve] of [
      [`wallet:${ownerId}:standard:secp256k1:0`, "secp256k1"],
      [`account:${ownerId}:standard:ed25519:0`, "ed25519"],
    ] as const)
      inventory.push({
        key_id,
        curve,
        owner: ownerId,
        public_key: "",
        epoch: 0,
        operations: ["generate"],
      });
    await publishInventory();
    const pending = await request("POST", "/v1/wallet/provision", {
      funding: "self",
    });
    assert.equal(pending.status, 503, json(pending.body));
    assert.equal(pending.body.error, "custody_authority_unavailable");
    const me = await request("GET", "/v1/wallet/me");
    assert.equal(me.status, 200, json(me.body));
    ownerAddress = address(record(me.body.wallet).address);
    await admitKeys(ownerId);
    const awaitingFunding = await request("POST", "/v1/wallet/provision", {
      funding: "self",
    });
    assert.equal(awaitingFunding.status, 200, json(awaitingFunding.body));
    assert.deepEqual(awaitingFunding.body.provisioning, {
      state: "bind_signed",
      awaiting: "owner",
    });
    assert.equal(await rpc.getBalance({ address: ownerAddress }), 0n);
    assert.equal(fixture.rpc.broadcasts.length, 0);
    const sponsored = await request("POST", "/v1/wallet/provision", {});
    assert.equal(sponsored.status, 503, json(sponsored.body));
    assert.equal(sponsored.body.error, "gas_station_unconfigured");
    assert.equal(fixture.rpc.broadcasts.length, 0);
    const fundedOwner = await fixtureSend(
      fundedWallet,
      ownerAddress,
      "0x",
      WAD,
    );
    assert.equal(await rpc.getBalance({ address: ownerAddress }), WAD);
    report(
      "owner funding",
      "fixture wallet",
      "eth_sendRawTransaction",
      { balance: WAD },
      fundedOwner,
    );
    const before = fixture.rpc.broadcasts.length;
    const provisioned = await request("POST", "/v1/wallet/provision", {
      funding: "self",
    });
    assert.equal(provisioned.status, 200, json(provisioned.body));
    assert.equal(record(provisioned.body.wallet).binding_state, "bound");
    assert.equal(record(provisioned.body.provisioning).state, "active");
    assert.equal(typeof provisioned.body.identityBinding, "string");
    const ownerBinding = await rpc.readContract({
      address: ADDR_PRECOMPILE,
      abi: addrAbi,
      functionName: "getUnifiedAccount",
      args: [ownerAddress],
    });
    assert.equal(
      `did:layerx:${ownerBinding[2].slice(2)}`,
      record(provisioned.body.wallet).did,
    );
    assert.equal(
      ownerBinding[3].slice(2),
      mainAccountId(String(record(provisioned.body.wallet).did)),
    );
    assert.equal(fixture.rpc.broadcasts.length - before, 1);
    const mined = await receipt(
      keccak256(fixture.rpc.broadcasts.at(-1)!.raw as Hex),
    );
    assert.ok(
      fixture.rpc.receiptRungs.some(
        (entry) =>
          entry.hash === mined.transactionHash && entry.rung === "committed",
      ),
    );
    const bound = parseEventLogs({
      abi: addrAbi,
      logs: mined.logs,
      eventName: "LayerXBound",
    });
    assert.equal(bound.length, 1);
    assert.equal(address(bound[0]!.args.evm), ownerAddress);
    assert.equal(bound[0]!.args.didPublicKey, ownerBinding[2]);
    report(
      "owner binding",
      "wallet provision",
      "/v1/wallet/provision",
      { binding: ownerBinding, state: "active" },
      mined,
    );
    const replay = await request("POST", "/v1/wallet/provision", {
      funding: "self",
    });
    assert.equal(replay.status, 200, json(replay.body));
    assert.equal(fixture.rpc.broadcasts.length - before, 1);

    await agent.authenticate();
    const registered = record(await agent.read("/v1/agent/me"));
    assert.equal(registered.owner_user_id, null);
    assert.equal(registered.is_frozen, true);
    assert.equal(registered.wallet, null);
    await owner(
      "POST",
      "/claim",
      buildClaim({ privateKey: seed, did: agent.did, ownerUserId: ownerId }),
    );
    const claimed = record(await agent.read("/v1/agent/me"));
    assert.equal(claimed.owner_user_id, ownerId);
    report(
      "register and claim",
      "authenticate",
      "/v1/agent/auth/challenge; /v1/agent/auth/verify; /v1/agents/:did/claim",
      { owner: claimed.owner_user_id, frozen: claimed.is_frozen },
    );
    await owner("POST", "/unfreeze", {});
    await owner("PUT", "/policy", {
      mode: "full",
      max_tx_value_wei: WAD.toString(),
      max_daily_value_wei: (10n * WAD).toString(),
      rate_limit_per_min: 600,
    });
    inventory.push({
      key_id: `wallet:${syntheticUser(agent.did)}:agent:secp256k1:0`,
      curve: "secp256k1",
      owner: `agent:${publicKey}`,
      public_key: "",
      epoch: 0,
      operations: ["generate"],
    });
    await publishInventory();
    const generated = record(await agent.provision(undefined, "self"));
    const provision = record(generated.provision);
    assert.equal(provision.state, "identity");
    assert.equal(provision.awaiting, "agent_signature");
    assert.equal(provision.did, `did:layerx:${publicKey}`);
    agentAddress = address(record(generated.wallet).address);
    assert.equal(
      (
        await rpc.readContract({
          address: ADDR_PRECOMPILE,
          abi: addrAbi,
          functionName: "getUnifiedAccount",
          args: [agentAddress],
        })
      )[2],
      `0x${"00".repeat(32)}`,
    );
    await admitKeys(`agent:${publicKey}`);
    const native = await owner("POST", "/fund", {
      amount: (WAD / 2n).toString(),
    });
    const nativeReceipt = await receipt(hash(native.tx_hash));
    assert.equal(await rpc.getBalance({ address: agentAddress }), WAD / 2n);
    assert.equal(address(native.from), ownerAddress);
    assert.equal(address(native.to), agentAddress);
    report(
      "agent native funding",
      "owner fund",
      `${ownerRoute}/fund`,
      { native: WAD / 2n },
      nativeReceipt,
    );
    assert.equal(typeof provision.bindMessage, "string");
    const signature = Buffer.from(
      ed25519.sign(
        Buffer.from(String(provision.bindMessage).slice(2), "hex"),
        seed,
      ),
    ).toString("hex");
    const count = fixture.rpc.broadcasts.length;
    const active = record(await agent.provision(signature, "self"));
    assert.equal(record(active.provision).state, "active");
    const binding = await rpc.readContract({
      address: ADDR_PRECOMPILE,
      abi: addrAbi,
      functionName: "getUnifiedAccount",
      args: [agentAddress],
    });
    assert.equal(binding[2], `0x${publicKey}`);
    assert.equal(binding[3].slice(2), mainAccountId(`did:layerx:${publicKey}`));
    assert.equal(fixture.rpc.broadcasts.length - count, 1);
    const bindingReceipt = await receipt(
      keccak256(fixture.rpc.broadcasts.at(-1)!.raw as Hex),
    );
    assert.ok(
      fixture.rpc.receiptRungs.some(
        (entry) =>
          entry.hash === bindingReceipt.transactionHash &&
          entry.rung === "committed",
      ),
    );
    const boundAgent = parseEventLogs({
      abi: addrAbi,
      logs: bindingReceipt.logs,
      eventName: "LayerXBound",
    });
    assert.equal(boundAgent.length, 1);
    assert.equal(address(boundAgent[0]!.args.evm), agentAddress);
    assert.equal(boundAgent[0]!.args.didPublicKey, `0x${publicKey}`);
    assert.equal(
      await rpc.readContract({
        address: ADDR_PRECOMPILE,
        abi: addrAbi,
        functionName: "layerXBindNonce",
        args: [agentAddress],
      }),
      1n,
    );
    report(
      "agent self-funded binding",
      "provision",
      "/v1/agent/provision",
      { binding, state: "active" },
      bindingReceipt,
    );
    const replayAgent = record(await agent.provision(signature, "self"));
    assert.equal(record(replayAgent.provision).state, "active");
    assert.equal(fixture.rpc.broadcasts.length - count, 1);
  }

  async function saveTradingPresetAndFundCollateral() {
    const preset = await buildTradingPreset({
      ownerAddress,
      maxTxValueWei: WAD,
      maxDailyValueWei: 10n * WAD,
      maxApproveWei: tokenCap,
      budgetWei: 1_000n * UNIT,
      expiresInSeconds: 3_600,
      rateLimitPerMin: 600,
    });
    const policy = await owner("PUT", "/policy", preset.policy);
    assert.deepEqual(record(policy.policy), preset.policy);
    for (const rule of preset.rules) {
      const saved = record((await owner("POST", "/rules", rule, 201)).rule);
      assert.equal(saved.subject, rule.subject);
      assert.equal(saved.effect, rule.effect);
      assert.equal(saved.value, rule.value);
      if (rule.subject === "token") {
        assert.equal(typeof saved.id, "string");
        tokenRuleId = String(saved.id);
      }
    }
    const budget = record(
      (await owner("POST", "/budgets", preset.budget, 201)).budget,
    );
    assert.equal(budget.cap_wei, preset.budget.cap_wei);
    assert.equal(String(budget.token).toLowerCase(), USDL.toLowerCase());
    assert.equal(
      array((await owner("GET", "/rules")).rules).length,
      preset.rules.length,
    );
    assert.equal(array((await owner("GET", "/budgets")).budgets).length, 1);
    report(
      "save preset",
      "buildTradingPreset",
      `${ownerRoute}/policy; /rules; /budgets`,
      { policy: policy.policy, rules: preset.rules, budget },
    );
    const ownerTokenBefore = await tokenBalance(ownerAddress);
    const transfer = await fixtureSend(
      fundedWallet,
      USDL,
      encodeFunctionData({
        abi: erc20Abi,
        functionName: "transfer",
        args: [ownerAddress, fundedTokens],
      }),
    );
    assert.equal(
      await tokenBalance(ownerAddress),
      ownerTokenBefore + fundedTokens,
    );
    const sourceEvents = parseEventLogs({
      abi: erc20Abi,
      logs: transfer.logs,
      eventName: "Transfer",
    });
    assert.equal(sourceEvents.length, 1);
    assert.equal(sourceEvents[0]!.args.value, fundedTokens);
    report(
      "owner collateral funding",
      "fixture wallet",
      "eth_sendRawTransaction",
      { ownerTokens: await tokenBalance(ownerAddress) },
      transfer,
    );
    const funding = await owner("POST", "/fund", {
      token: USDL,
      amount: fundedTokens.toString(),
    });
    const mined = await receipt(hash(funding.tx_hash));
    const transfers = parseEventLogs({
      abi: erc20Abi,
      logs: mined.logs,
      eventName: "Transfer",
    });
    assert.equal(transfers.length, 1);
    assert.equal(address(transfers[0]!.args.from), ownerAddress);
    assert.equal(address(transfers[0]!.args.to), agentAddress);
    assert.equal(transfers[0]!.args.value, fundedTokens);
    assert.equal(await tokenBalance(), fundedTokens);
    assert.equal(await tokenBalance(ownerAddress), ownerTokenBefore);
    report(
      "agent collateral funding",
      "owner fund",
      `${ownerRoute}/fund`,
      {
        walletTokens: await tokenBalance(),
        ownerTokens: await tokenBalance(ownerAddress),
      },
      mined,
    );
  }

  async function marketsReadsWithBoundedConcurrency() {
    fixture.rpc.beginReadObservation();
    const markets = array(await tool("markets")).map(record);
    const response = await fetch(new URL("graphql", dataUrl), {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: json({ query: "{ markets { marketId } }" }),
    });
    assert.equal(response.status, 200);
    const listed = array(record(record(await response.json()).data).markets);
    assert.deepEqual(
      markets.map((item) => Number(record(item.config).marketId)),
      listed.map((item) => record(item).marketId),
    );
    assert.ok(fixture.rpc.maxConcurrentReads <= 2);
    const market = markets.find(
      (item) => record(item.config).marketId === marketId,
    );
    assert.ok(market);
    assert.equal(record(market.mark).markPrice, WAD);
    assert.equal(record(market.mark).fresh, true);
    assert.equal(record(market.config).lotSize, WAD);
    const account = await accountAgreement();
    assert.equal(account.collateral, 0n);
    assert.equal(account.positionCount, 0n);
    assert.equal(account.openOrderCount, 0n);
    report(
      "markets and account",
      "markets; account",
      "/graphql; eth_call getMarketConfig/getAccount",
      { market, account },
    );
  }

  async function attestorAuthorizationResumesDepositAfterGatewayRestart() {
    const before = await noBroadcast();
    const custodyBefore = await tokenBalance(PERPS_CUSTODY);
    const accepted = record(
      await tool("deposit", {
        amount: formatUnits(depositAmount, 6),
        idempotencyKey: `deposit-${randomUUID()}`,
      }),
    );
    assert.equal(accepted.terminal, false);
    assert.equal(accepted.approval, null);
    assert.equal(accepted.call, null);
    assert.equal(typeof accepted.action_id, "string");
    depositActionId = String(accepted.action_id);
    // Reads on the agent lane expose the pending signed intent without authorizing it.
    const deadline = Date.now() + 120_000;
    for (;;) {
      assert.ok(readAuthorization);
      const response = await fetch(
        `${fixture.url}/v1/agent/actions/${depositActionId}`,
        {
          headers: { authorization: readAuthorization },
          signal: AbortSignal.timeout(30_000),
        },
      );
      assert.equal(response.status, 200);
      assert.equal(
        response.headers.get("x-served-by"),
        "paxeer-wallet-gateway",
      );
      const pending = record(await response.json());
      if (pending.error !== null) {
        assert.equal(
          record(pending.error).code,
          "AGENT_REAUTHORIZATION_REQUIRED",
          json(pending),
        );
        break;
      }
      assert.equal(pending.terminal, false, json(pending));
      assert.ok(Date.now() < deadline, json(pending));
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    assert.deepEqual(await noBroadcast(), before);
    await fixture.restart();
    let done: Record<string, unknown>;
    const completedBy = Date.now() + 300_000;
    for (;;) {
      done = record(await tool("action_status", { actionId: depositActionId }));
      assert.equal(done.action_id, depositActionId);
      if (done.terminal === true) break;
      assert.ok(Date.now() < completedBy, json(done));
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    assert.equal(done.status, "confirmed");
    assert.equal(done.ok, true);
    assert.equal(done.credit_verified, true);
    const approval = await gatewayReceipt(hash(record(done.approval).tx_hash));
    const deposit = await gatewayReceipt(hash(record(done.call).tx_hash));
    for (const mined of [approval, deposit])
      assert.ok(
        fixture.rpc.receiptRungs.some(
          (entry) =>
            entry.hash === mined.transactionHash && entry.rung === "committed",
        ),
      );
    const approved = parseEventLogs({
      abi: erc20Abi,
      logs: approval.logs,
      eventName: "Approval",
    });
    assert.equal(approved.length, 1);
    assert.equal(address(approved[0]!.args.owner), agentAddress);
    assert.equal(address(approved[0]!.args.spender), PERPS);
    assert.equal(approved[0]!.args.value, depositAmount);
    const deposited = event(deposit, "CollateralDeposited").args;
    assert.equal(address(deposited.account), agentAddress);
    assert.equal(deposited.amount, depositAmount);
    assert.equal(deposited.newBalance, depositAmount);
    const moved = parseEventLogs({
      abi: erc20Abi,
      logs: deposit.logs,
      eventName: "Transfer",
    });
    assert.equal(moved.length, 1);
    assert.equal(address(moved[0]!.address), USDL);
    assert.equal(address(moved[0]!.args.from), agentAddress);
    assert.equal(address(moved[0]!.args.to), PERPS_CUSTODY);
    assert.equal(moved[0]!.args.value, depositAmount);
    assert.equal(
      await tokenBalance(PERPS_CUSTODY),
      custodyBefore + depositAmount,
    );
    const account = await accountAgreement();
    assert.equal(account.collateral, depositAmount);
    assert.equal(await tokenBalance(), fundedTokens - depositAmount);
    assert.equal(await agent.allowance(USDL, PERPS), 0n);
    assert.equal(fixture.rpc.broadcasts.length - before.sends, 2);
    assert.deepEqual(
      fixture.rpc.broadcasts
        .slice(before.sends)
        .map((entry) => keccak256(Buffer.from(entry.raw.slice(2), "hex"))),
      [approval.transactionHash, deposit.transactionHash],
    );
    const replay = record(
      await tool("action_status", { actionId: depositActionId }),
    );
    assert.deepEqual(replay.approval, done.approval);
    assert.deepEqual(replay.call, done.call);
    assert.equal(fixture.rpc.broadcasts.length - before.sends, 2);
    report(
      "deposit approval",
      "deposit; action_status",
      "/v1/agent/actions/allowance-and-call; /v1/agent/actions/:id",
      { allowanceConsumed: true, amount: depositAmount },
      approval,
    );
    report(
      "deposit after restart",
      "action_status",
      "/v1/agent/actions/:id",
      {
        account,
        walletTokens: await tokenBalance(),
        actionId: depositActionId,
      },
      deposit,
    );
  }

  async function limitOrderIsVisibleAndCanBeCancelled() {
    const mined = await write("place_limit", {
      marketId: marketId.toString(),
      isLong: true,
      sizeUsd: "20",
      leverage: "2",
      limitPrice: "0.5",
      timeInForce: "POST_ONLY",
      clientOrderId: `0x${randomBytes(16).toString("hex")}`,
    });
    const placed = event(mined, "OrderPlaced").args;
    orderId = placed.orderId;
    assert.equal(address(placed.user), agentAddress);
    assert.equal(placed.marketId, marketId);
    assert.equal(placed.isLong, true);
    assert.equal(placed.sizeUsd, 20n * WAD);
    const orders = array(await tool("open_orders")).map(record);
    assert.equal(orders.length, 1);
    assert.equal(orders[0]!.orderId, orderId);
    assert.equal(orders[0]!.limitPrice, WAD / 2n);
    assert.deepEqual(orders[0], { orderId, ...(await venue.order(orderId)) });
    assert.equal((await accountAgreement()).openOrderCount, 1n);
    report(
      "limit order",
      "place_limit; open_orders",
      "/v1/agent/simulate; /v1/agent/send; eth_call getOrder",
      { order: orders[0] },
      mined,
    );
    const cancelled = await write("cancel_order", {
      orderId: orderId.toString(),
    });
    assert.equal(event(cancelled, "OrderCancelled").args.orderId, orderId);
    assert.equal((await venue.order(orderId)).active, false);
    assert.deepEqual(await tool("open_orders"), []);
    assert.equal((await accountAgreement()).openOrderCount, 0n);
    report(
      "cancel limit",
      "cancel_order; open_orders",
      "/v1/agent/simulate; /v1/agent/send",
      { orderId, active: false },
      cancelled,
    );
  }

  async function makerLiquidityAndMarketPosition() {
    const balanceBefore = await tokenBalance(maker.address);
    const approval = await fixtureSend(
      makerWallet,
      USDL,
      encodeFunctionData({
        abi: erc20Abi,
        functionName: "approve",
        args: [PERPS, 10_000n * UNIT],
      }),
    );
    assert.equal(
      parseEventLogs({
        abi: erc20Abi,
        logs: approval.logs,
        eventName: "Approval",
      })[0]!.args.value,
      10_000n * UNIT,
    );
    report(
      "maker approval",
      "fixture wallet",
      "eth_sendRawTransaction",
      {
        allowance: await rpc.readContract({
          address: USDL,
          abi: erc20Abi,
          functionName: "allowance",
          args: [maker.address, PERPS],
        }),
      },
      approval,
    );
    const deposited = await makerCall(calls.buildDeposit(10_000n * UNIT));
    assert.equal(
      event(deposited, "CollateralDeposited").args.amount,
      10_000n * UNIT,
    );
    assert.equal(
      await tokenBalance(maker.address),
      balanceBefore - 10_000n * UNIT,
    );
    report(
      "maker collateral",
      "fixture wallet",
      "eth_sendRawTransaction",
      {
        collateral: (
          await rpc.readContract(calls.buildGetAccount(maker.address))
        ).collateral,
      },
      deposited,
    );
    const makerOrder = await liquidity(false);
    const quoted = await venue.quoteOpen(
      marketId,
      100n * UNIT,
      2n * WAD,
      true,
      agentAddress,
    );
    assert.equal(quoted.sizeUsd, 198n * WAD);
    assert.equal(quoted.entryPrice, WAD);
    const mined = await write("open_market", {
      marketId: marketId.toString(),
      isLong: true,
      collateralAmount: "100",
      leverage: "2",
    });
    const opened = events(mined).filter(
      (item) =>
        item.eventName === "PositionOpened" &&
        address(item.args.user) === agentAddress,
    );
    assert.equal(opened.length, 1);
    const args = opened[0]!.args;
    positionId = args.positionId;
    assert.equal(args.marketId, marketId);
    assert.equal(args.isLong, true);
    assert.equal(args.sizeUsd, quoted.sizeUsd);
    assert.equal(args.entryPrice, WAD);
    const direct = positionFromTuple(
      await rpc.readContract(calls.buildGetPosition(positionId)),
    );
    assert.equal(direct.active, true);
    assert.equal(direct.user.toLowerCase(), agentAddress.toLowerCase());
    assert.equal(direct.sizeUsd, quoted.sizeUsd);
    assert.equal(direct.entryPrice, WAD);
    const fill = event(mined, "Fill").args;
    assert.equal(fill.makerOrderId, makerOrder);
    assert.equal(address(fill.maker), maker.address);
    assert.equal(address(fill.taker), agentAddress);
    assert.equal(fill.sizeUsd, quoted.sizeUsd);
    const makerDetails = await rpc.readContract({
      address: PERPS,
      abi: PerpsPrecompileABI,
      functionName: "getOrderDetails",
      args: [makerOrder],
    });
    assert.equal(makerDetails.sizeBase, 1_000n * WAD);
    assert.equal(makerDetails.filledBase, quoted.sizeUsd);
    const positions = array(await tool("positions")).map(record);
    assert.equal(positions.length, 1);
    assert.equal(positions[0]!.positionId, positionId);
    for (const [key, value] of Object.entries(direct))
      assert.equal(positions[0]![key], value);
    assert.equal((await accountAgreement()).positionCount, 1n);
    accountSettlements(mined);
    report(
      "market fill",
      "open_market; positions",
      "/v1/agent/simulate; /v1/agent/send; eth_call getPosition",
      {
        positionId,
        position: direct,
        quote: quoted,
        makerOrderRemaining: makerDetails.sizeBase - makerDetails.filledBase,
      },
      mined,
    );
  }

  function accountSettlements(mined: TransactionReceipt) {
    for (const item of events(mined)) {
      if (
        item.eventName === "FeeCollected" &&
        address(item.args.user) === agentAddress
      )
        feeTokens += item.args.feeTokens;
      if (
        item.eventName === "TradeSettled" &&
        address(item.args.user) === agentAddress
      ) {
        grossPnl += item.args.grossPnl;
        fundingPaid += item.args.fundingPaidUsd;
        settlementDelta +=
          (item.args.grossPnl + item.args.fundingPaidUsd) / (WAD / UNIT);
      }
    }
  }

  async function positionProtectionAndPartialClose() {
    const mined = await write("set_tpsl", {
      positionId: positionId.toString(),
      takeProfit: "1.2",
      stopLoss: "0.8",
    });
    const protection = event(mined, "PositionTpSlSet").args;
    assert.equal(protection.positionId, positionId);
    assert.equal(protection.takeProfitPrice, (12n * WAD) / 10n);
    assert.equal(protection.stopLossPrice, (8n * WAD) / 10n);
    const details = await rpc.readContract(
      calls.buildGetPositionDetails(positionId),
    );
    assert.equal(details.takeProfitPrice, protection.takeProfitPrice);
    assert.equal(details.stopLossPrice, protection.stopLossPrice);
    report(
      "take profit and stop loss",
      "set_tpsl; positions",
      "/v1/agent/simulate; /v1/agent/send",
      { positionId, details },
      mined,
    );
    await liquidity(true);
    const partial = await write("partial_close", {
      positionId: positionId.toString(),
      fractionBps: 5000,
    });
    const closed = events(partial).filter(
      (item) =>
        item.eventName === "PositionClosed" &&
        item.args.positionId === positionId,
    );
    assert.equal(closed.length, 1);
    assert.equal(closed[0]!.args.isFullClose, false);
    assert.equal(closed[0]!.args.closedSizeUsd, 99n * WAD);
    assert.equal(closed[0]!.args.exitPrice, WAD);
    const remaining = await venue.position(positionId);
    assert.equal(remaining.sizeUsd, 99n * WAD);
    assert.equal(remaining.active, true);
    assert.equal(array(await tool("positions")).length, 1);
    accountSettlements(partial);
    report(
      "partial close",
      "partial_close; positions",
      "/v1/agent/simulate; /v1/agent/send",
      { positionId, remaining, account: await accountAgreement() },
      partial,
    );
  }

  async function closeRemainderAndWithdraw() {
    await liquidity(true);
    const mined = await write("close_position", {
      positionId: positionId.toString(),
    });
    const closed = events(mined).filter(
      (item) =>
        item.eventName === "PositionClosed" &&
        item.args.positionId === positionId,
    );
    assert.equal(closed.length, 1);
    assert.equal(closed[0]!.args.isFullClose, true);
    assert.equal(closed[0]!.args.closedSizeUsd, 99n * WAD);
    assert.equal(closed[0]!.args.exitPrice, WAD);
    assert.deepEqual(
      await rpc.readContract(calls.buildGetUserPositionIds(agentAddress)),
      [],
    );
    assert.deepEqual(await tool("positions"), []);
    accountSettlements(mined);
    const before = await accountAgreement();
    assert.equal(before.positionCount, 0n);
    assert.equal(before.openOrderCount, 0n);
    assert.equal(before.withdrawable, before.collateral);
    report(
      "close remainder",
      "close_position; positions",
      "/v1/agent/simulate; /v1/agent/send",
      { positionIds: [], account: before },
      mined,
    );
    const tokensBefore = await tokenBalance();
    const withdrawal = await write("withdraw", {
      amount: formatUnits(before.withdrawable, 6),
    });
    const withdrawn = event(withdrawal, "CollateralWithdrawn").args;
    assert.equal(address(withdrawn.account), agentAddress);
    assert.equal(withdrawn.amount, before.withdrawable);
    assert.equal(withdrawn.newBalance, 0n);
    assert.equal(await tokenBalance(), tokensBefore + before.withdrawable);
    const after = await accountAgreement();
    assert.equal(after.collateral, 0n);
    assert.equal(after.withdrawable, 0n);
    assert.ok(feeTokens > 0n);
    assert.equal(grossPnl, 0n);
    assert.equal(
      await tokenBalance(),
      fundedTokens + settlementDelta - feeTokens,
    );
    report(
      "withdraw and reconcile",
      "withdraw; account",
      "/v1/agent/simulate; /v1/agent/send",
      {
        account: after,
        walletTokens: await tokenBalance(),
        feeTokens,
        grossPnl,
        fundingPaid,
      },
      withdrawal,
    );
  }

  async function perpsDepositCapChecksExistingAllowance() {
    const approval = await gatewayReceipt(
      hash(
        record(
          await agent.send({
            to: USDL,
            value: "0",
            data: encodeFunctionData({
              abi: erc20Abi,
              functionName: "approve",
              args: [PERPS, tokenCap],
            }),
          }),
        ).tx_hash,
      ),
    );
    assert.equal(
      parseEventLogs({
        abi: erc20Abi,
        logs: approval.logs,
        eventName: "Approval",
      })[0]!.args.value,
      tokenCap,
    );
    assert.equal(await agent.allowance(USDL, PERPS), tokenCap);
    report(
      "bounded approval",
      "send",
      "/v1/agent/send",
      { allowance: tokenCap },
      approval,
    );
    await owner("DELETE", `/rules/${tokenRuleId}`);
    const tightenedCap = 400n * UNIT;
    const tightened = record(
      (
        await owner(
          "POST",
          "/rules",
          {
            effect: "allow",
            subject: "token",
            value: USDL,
            max_value_wei: tightenedCap.toString(),
          },
          201,
        )
      ).rule,
    );
    assert.equal(tightened.max_value_wei, tightenedCap.toString());
    const overCap = tightenedCap + 1n;
    const tx: WireTransaction = {
      to: PERPS,
      data: encodeFunctionData(calls.buildDeposit(overCap)),
      value: "0",
    };
    assert.equal(record(await agent.simulate(tx)).ok, true);
    await refusal(
      "deposit above transaction cap",
      () => agent.send(tx),
      "TOKEN_VALUE_CAP",
      403,
    );
    const before = await noBroadcast();
    const refused = await kit.callTool("deposit", {
      amount: formatUnits(overCap, 6),
      idempotencyKey: `overcap-${randomUUID()}`,
    });
    assert.equal(refused.ok, false, json(refused));
    assert.ok(!refused.ok);
    assert.equal(refused.error.code, "TOKEN_VALUE_CAP");
    assert.equal(record(refused.error.details).status, 403);
    report(
      "kit deposit above cap",
      "deposit",
      "/v1/agent/simulate; /v1/agent/send",
      {
        status: 403,
        code: refused.error.code,
        allowance: tokenCap,
        cap: tightenedCap,
        amount: overCap,
      },
    );
    assert.deepEqual(await noBroadcast(), before);
    await refusal(
      "contract outside preset",
      () => agent.send({ to: ADDR_PRECOMPILE, data: tx.data, value: "0" }),
      "CONTRACT_NOT_ALLOWLISTED",
      403,
    );
  }

  async function freezeRefusesOrderAndUnfreezeRestoresTrading() {
    const walletBefore = await tokenBalance();
    const resumedCollateral = 50n * UNIT;
    const deposited = await write("deposit", {
      amount: formatUnits(resumedCollateral, 6),
      idempotencyKey: `resume-${randomUUID()}`,
    });
    assert.equal(
      event(deposited, "CollateralDeposited").args.amount,
      resumedCollateral,
    );
    assert.equal((await accountAgreement()).collateral, resumedCollateral);
    assert.equal(await tokenBalance(), walletBefore - resumedCollateral);
    report(
      "collateral for resumed trading",
      "deposit; account",
      "/v1/agent/send",
      {
        collateral: resumedCollateral,
        walletTokens: await tokenBalance(),
      },
      deposited,
    );
    const frozen = await owner("POST", "/freeze", {});
    assert.equal(frozen.is_frozen, true);
    assert.equal(record(await agent.read("/v1/agent/me")).is_frozen, true);
    const input = {
      marketId: marketId.toString(),
      isLong: true,
      sizeUsd: "20",
      leverage: "2",
      limitPrice: "0.5",
      timeInForce: "POST_ONLY",
      clientOrderId: `0x${randomBytes(16).toString("hex")}`,
    };
    const before = await noBroadcast();
    const refused = await kit.callTool("place_limit", input);
    assert.equal(refused.ok, false, json(refused));
    assert.ok(!refused.ok);
    assert.equal(refused.error.code, "agent_frozen");
    assert.equal(record(refused.error.details).status, 403);
    assert.deepEqual(await noBroadcast(), before);
    report("frozen order refusal", "place_limit", "/v1/agent/send", {
      status: 403,
      code: refused.error.code,
      unchanged: before,
    });
    const resumed = await owner("POST", "/unfreeze", {});
    assert.equal(resumed.is_frozen, false);
    const mined = await write("place_limit", input);
    const id = event(mined, "OrderPlaced").args.orderId;
    assert.equal((await venue.order(id)).active, true);
    assert.equal(array(await tool("open_orders")).length, 1);
    report(
      "unfrozen order",
      "place_limit; open_orders",
      "/v1/agent/send",
      { orderId: id, order: await venue.order(id) },
      mined,
    );
    const cancelled = await write("cancel_order", { orderId: id.toString() });
    assert.equal(event(cancelled, "OrderCancelled").args.orderId, id);
    assert.deepEqual(await tool("open_orders"), []);
    report(
      "cancel unfrozen order",
      "cancel_order",
      "/v1/agent/send",
      { orderId: id, active: (await venue.order(id)).active },
      cancelled,
    );
    const withdrawn = await write("withdraw", {
      amount: formatUnits(resumedCollateral, 6),
    });
    assert.equal(
      event(withdrawn, "CollateralWithdrawn").args.amount,
      resumedCollateral,
    );
    assert.equal((await accountAgreement()).collateral, 0n);
    assert.equal(await tokenBalance(), walletBefore);
    report(
      "withdraw resumed collateral",
      "withdraw; account",
      "/v1/agent/send",
      {
        collateral: 0n,
        walletTokens: await tokenBalance(),
      },
      withdrawn,
    );
  }

  async function sweepRemainderAndReadActivity() {
    const tokens = await tokenBalance();
    const ownerTokens = await tokenBalance(ownerAddress);
    const swept = await owner("POST", "/sweep", {
      token: USDL,
      amount: tokens.toString(),
    });
    const mined = await receipt(hash(swept.tx_hash), true);
    assert.equal(address(swept.from), agentAddress);
    assert.equal(address(swept.to), ownerAddress);
    const transfers = parseEventLogs({
      abi: erc20Abi,
      logs: mined.logs,
      eventName: "Transfer",
    });
    assert.equal(transfers.length, 1);
    assert.equal(address(transfers[0]!.args.from), agentAddress);
    assert.equal(address(transfers[0]!.args.to), ownerAddress);
    assert.equal(transfers[0]!.args.value, tokens);
    assert.equal(await tokenBalance(), 0n);
    assert.equal(await tokenBalance(ownerAddress), ownerTokens + tokens);
    report(
      "sweep collateral",
      "owner sweep",
      `${ownerRoute}/sweep`,
      { agentTokens: 0n, ownerTokens: await tokenBalance(ownerAddress) },
      mined,
    );
    const nativeBefore = await rpc.getBalance({ address: agentAddress });
    const ownerNative = await rpc.getBalance({ address: ownerAddress });
    const reserve = WAD / 100n;
    assert.ok(nativeBefore > reserve);
    const amount = nativeBefore - reserve;
    const nativeSweep = await owner("POST", "/sweep", {
      amount: amount.toString(),
    });
    const nativeReceipt = await receipt(hash(nativeSweep.tx_hash), true);
    const nativeAfter = await rpc.getBalance({ address: agentAddress });
    assert.equal(
      await rpc.getBalance({ address: ownerAddress }),
      ownerNative + amount,
    );
    assert.equal(
      nativeAfter,
      reserve - nativeReceipt.gasUsed * nativeReceipt.effectiveGasPrice,
    );
    assert.ok(nativeAfter >= 0n && nativeAfter < reserve);
    report(
      "sweep native with gas reserve",
      "owner sweep",
      `${ownerRoute}/sweep`,
      {
        swept: amount,
        agentNative: nativeAfter,
        ownerNative: ownerNative + amount,
      },
      nativeReceipt,
    );
    const ownerActivity = array((await owner("GET", "/activity")).events).map(
      record,
    );
    const agentActivity = array(
      record(await agent.read("/v1/agent/activity")).events,
    ).map(record);
    assert.deepEqual(agentActivity, ownerActivity);
    const recorded = new Set(ownerActivity.map((entry) => entry.tx_hash));
    for (const tx of agentTransactions)
      assert.ok(recorded.has(tx), `activity contains ${tx}`);
    assert.equal(recorded.size, agentTransactions.size);
    assert.equal(ownerActivity.length, agentTransactions.size);
    for (const entry of ownerActivity) {
      assert.equal(entry.kind, "transaction");
      assert.ok(receipts.has(hash(entry.tx_hash)));
    }
    const account = await accountAgreement();
    assert.equal(account.collateral, 0n);
    assert.equal(account.positionCount, 0n);
    assert.equal(account.openOrderCount, 0n);
    report(
      "activity and final account",
      "read; account",
      `${ownerRoute}/activity; /v1/agent/activity; eth_call getAccount`,
      { activity: ownerActivity, account, walletTokens: await tokenBalance() },
    );
  }

  await run(selfFundedBindingWaitsForCommittedState);
  await run(saveTradingPresetAndFundCollateral);
  await run(marketsReadsWithBoundedConcurrency);
  await run(attestorAuthorizationResumesDepositAfterGatewayRestart);
  await run(limitOrderIsVisibleAndCanBeCancelled);
  await run(makerLiquidityAndMarketPosition);
  await run(positionProtectionAndPartialClose);
  await run(closeRemainderAndWithdraw);
  await run(perpsDepositCapChecksExistingAllowance);
  await run(freezeRefusesOrderAndUnfreezeRestoresTrading);
  await run(sweepRemainderAndReadActivity);
}

test(localAgentJourney);
