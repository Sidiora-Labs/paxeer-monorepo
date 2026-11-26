import assert from "node:assert/strict";
import { after, before, test, type TestContext } from "node:test";
import { spawn, type ChildProcess } from "node:child_process";
import { randomBytes } from "node:crypto";
import {
  cp,
  mkdir,
  mkdtemp,
  readFile,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import { createRequire } from "node:module";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  AGENT_ENV,
  describeAgentInterface,
  type AgentWorkspaceInterface,
} from "../packages/agent-kit/src/interface";

const root = fileURLToPath(new URL("../", import.meta.url));
const kitRequire = createRequire(
  path.join(root, "packages/agent-kit/package.json"),
);
const { Client } = kitRequire("@modelcontextprotocol/sdk/client/index.js");
const { StdioClientTransport } = kitRequire(
  "@modelcontextprotocol/sdk/client/stdio.js",
);
let web: ChildProcess | undefined;
let origin: string;
let temporary: string;
let keyFile: string;
let seed: string;
let webOutput = "";

before(
  async () => {
    temporary = await mkdtemp(path.join(tmpdir(), "perps-agent-connect-"));
    keyFile = path.join(temporary, "agent-seed");
    seed = randomBytes(32).toString("hex");
    await writeFile(keyFile, seed, { mode: 0o600, flag: "wx" });
    const application = path.join(temporary, "apps/web");
    await mkdir(application, { recursive: true });
    for (const name of [
      "src",
      "next.config.js",
      "package.json",
      "tsconfig.json",
      "next-env.d.ts",
      "postcss.config.js",
      "tailwind.config.js",
    ]) {
      await cp(
        path.join(root, "apps/web", name),
        path.join(application, name),
        { recursive: true },
      );
    }
    await cp(
      path.join(root, "tsconfig.json"),
      path.join(temporary, "tsconfig.json"),
    );
    await symlink(
      path.join(root, "packages"),
      path.join(temporary, "packages"),
      "dir",
    );
    await symlink(
      path.join(root, "node_modules"),
      path.join(temporary, "node_modules"),
      "dir",
    );
    await symlink(
      path.join(root, "apps/web/node_modules"),
      path.join(application, "node_modules"),
      "dir",
    );
    await symlink(
      path.join(root, "apps/web/public"),
      path.join(application, "public"),
      "dir",
    );
    web = spawn(
      process.execPath,
      [
        "-e",
        `
      const http = require("node:http");
      const next = require("next");
      let app;
      const server = http.createServer((req, res) => app.getRequestHandler()(req, res));
      server.listen(0, "127.0.0.1", async () => {
        try {
          const origin = "http://127.0.0.1:" + server.address().port;
          Object.assign(process.env, {
            NEXT_PUBLIC_PAXEER_WALLET_API: origin + "/wallet",
            NEXT_PUBLIC_PAXEER_RPC_URLS: origin + "/rpc",
            NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
            NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
            NEXT_PUBLIC_PAXSCAN_API_URL: origin,
            NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http:", "ws:"),
          });
          app = next({ dev: true, dir: "apps/web" });
          await app.prepare();
          process.send({ origin });
        } catch (error) { console.error(error); process.exit(1); }
      });
      process.on("SIGTERM", async () => {
        server.closeAllConnections();
        server.close();
        if (app) await app.close();
        process.exit(0);
      });
    `,
      ],
      {
        cwd: temporary,
        env: {
          ...process.env,
          NODE_ENV: "development",
          NEXT_TELEMETRY_DISABLED: "1",
          PAXEER_NETWORK_BUILD_DIR: ".next-agent-connect",
        },
        stdio: ["ignore", "pipe", "pipe", "ipc"],
      },
    );
    web.stdout!.on("data", (chunk) => {
      webOutput += chunk;
    });
    web.stderr!.on("data", (chunk) => {
      webOutput += chunk;
    });
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error(`Web startup timed out: ${webOutput}`)),
        90_000,
      );
      web!.once("error", (error) => {
        clearTimeout(timer);
        reject(error);
      });
      web!.once("exit", (code) => {
        clearTimeout(timer);
        reject(new Error(`Web exited with ${code}: ${webOutput}`));
      });
      web!.once("message", (message) => {
        clearTimeout(timer);
        assert.ok(
          message && typeof message === "object" && "origin" in message,
        );
        assert.equal(typeof message.origin, "string");
        origin = message.origin as string;
        Object.assign(process.env, {
          NEXT_PUBLIC_PAXEER_WALLET_API: origin + "/wallet",
          NEXT_PUBLIC_PAXEER_RPC_URLS: origin + "/rpc",
          NEXT_PUBLIC_PERPS_GATEWAY_URL: origin,
          NEXT_PUBLIC_PAXSCAN_BASE_URL: origin,
          NEXT_PUBLIC_PAXSCAN_API_URL: origin,
          NEXT_PUBLIC_PAXSCAN_WS_URL: origin.replace("http:", "ws:"),
        });
        resolve();
      });
    });
  },
  { timeout: 100_000 },
);

after(async () => {
  if (web && web.exitCode === null) {
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => web!.kill("SIGKILL"), 10_000);
      web!.once("exit", () => {
        clearTimeout(timer);
        resolve();
      });
      web!.kill("SIGTERM");
    });
  }
  if (temporary) await rm(temporary, { recursive: true, force: true });
});

async function readInterface(): Promise<AgentWorkspaceInterface> {
  const response = await fetch(`${origin}/agents/interface.json`);
  assert.equal(
    response.status,
    200,
    `Agent interface response: ${await response.clone().text()}\n${webOutput}`,
  );
  assert.match(response.headers.get("content-type")!, /^application\/json/);
  assert.equal(response.headers.get("set-cookie"), null);
  assert.equal(response.headers.get("cache-control"), "no-store");
  return response.json();
}

async function agentInterfaceDocumentMatchesTheKit(t: TestContext) {
  const document = await readInterface();
  const description = describeAgentInterface();
  const { connection, ...servedKit } = document;
  assert.deepEqual(servedKit, JSON.parse(JSON.stringify(description)));
  const packageInfo = JSON.parse(
    await readFile(path.join(root, "packages/agent-kit/package.json"), "utf8"),
  );
  assert.equal(document.name, packageInfo.name);
  assert.equal(document.version, packageInfo.version);
  assert.deepEqual(connection, {
    walletGatewayUrl: `${origin}/wallet`,
    nodeUrl: `${origin}/rpc`,
    dataGatewayUrl: origin,
    chain: { id: 125, name: "Paxeer Mainnet" },
    venue: {
      contract:
        process.env.NEXT_PUBLIC_PERPS_PRECOMPILE ||
        "0x0000000000000000000000000000000000001015",
    },
  });
  assert.doesNotMatch(
    JSON.stringify(description),
    /"(?:walletGatewayUrl|nodeUrl|dataGatewayUrl)"/,
  );
  assert.ok(!JSON.stringify(description).includes(origin));
  assert.ok(!JSON.stringify(document).includes(seed));
  assert.ok(!JSON.stringify(document).includes(keyFile));
  assert.deepEqual(Object.keys(document).sort(), [
    "connection",
    "entry",
    "environment",
    "name",
    "tools",
    "version",
  ]);
  assert.deepEqual(
    document.environment.map((entry) => entry.name),
    Object.values(AGENT_ENV),
  );
  const cliSource = await readFile(
    path.join(root, "packages/agent-kit/src/mcp-cli.ts"),
    "utf8",
  );
  const required = [...cliSource.matchAll(/required\(AGENT_ENV\.(\w+)\)/g)].map(
    (match) => AGENT_ENV[match[1] as keyof typeof AGENT_ENV],
  );
  assert.deepEqual(
    document.environment
      .filter((entry) => entry.required)
      .map((entry) => entry.name),
    required,
  );
  assert.match(cliSource, /process\.env\[AGENT_ENV\.label\]/);
  assert.deepEqual(
    document.environment.filter((entry) => !entry.required),
    [{ name: AGENT_ENV.label, required: false, default: "perps" }],
  );

  const env = {
    ...process.env,
    [AGENT_ENV.walletGateway]: connection.walletGatewayUrl,
    [AGENT_ENV.node]: connection.nodeUrl,
    [AGENT_ENV.dataGateway]: connection.dataGatewayUrl,
    [AGENT_ENV.keyFile]: keyFile,
    [AGENT_ENV.label]: "connect",
  };
  const transport = new StdioClientTransport({
    command: document.entry.command,
    args: document.entry.args,
    cwd: path.resolve(root, "../..", document.entry.cwd),
    env,
    stderr: "pipe",
  });
  const client = new Client({
    name: "perps-interface-reader",
    version: "1.0.0",
  });
  try {
    await client.connect(transport);
    assert.deepEqual(client.getServerVersion(), {
      name: document.name,
      version: document.version,
    });
    const listed = await client.listTools();
    assert.equal(listed.tools.length, 18);
    assert.deepEqual(
      listed.tools.map(
        (tool: {
          name: string;
          description: string;
          inputSchema: object;
          annotations: { readOnlyHint: boolean };
        }) => ({
          name: tool.name,
          description: tool.description,
          inputSchema: tool.inputSchema,
          readOnly: tool.annotations.readOnlyHint,
        }),
      ),
      document.tools,
    );
    const { createAgentTools, AgentGatewayClient, PerpsDataClient } =
      await import("../packages/agent-kit/src/index");
    const gateway = new AgentGatewayClient({
      url: connection.walletGatewayUrl,
      privateKey: Buffer.from(seed, "hex"),
      label: "connect",
    });
    const tools = createAgentTools(
      gateway,
      new PerpsDataClient({
        nodeUrl: connection.nodeUrl,
        dataUrl: connection.dataGatewayUrl,
      }),
    );
    assert.deepEqual(
      tools.tools.map((tool) => tool.name),
      document.tools.map((tool) => tool.name),
    );
  } finally {
    await client.close();
    await transport.close();
  }

  const addressFreeEnvironment = { ...process.env };
  for (const name of Object.keys(addressFreeEnvironment)) {
    if (name.startsWith("NEXT_PUBLIC_") || name.startsWith("PAXEER_AGENT_"))
      delete addressFreeEnvironment[name];
  }
  const withoutConfiguration = spawn(
    process.execPath,
    [
      "--import",
      "tsx",
      "--input-type=module",
      "-e",
      'import { describeAgentInterface } from "@paxeer-network/agent-kit/interface"; process.stdout.write(JSON.stringify(describeAgentInterface()));',
    ],
    {
      cwd: root,
      env: addressFreeEnvironment,
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  let output = "";
  let errors = "";
  withoutConfiguration.stdout.on("data", (chunk) => {
    output += chunk;
  });
  withoutConfiguration.stderr.on("data", (chunk) => {
    errors += chunk;
  });
  const exitCode = await new Promise((resolve, reject) => {
    withoutConfiguration.once("error", reject);
    withoutConfiguration.once("exit", resolve);
  });
  assert.equal(exitCode, 0, errors);
  assert.deepEqual(JSON.parse(output), servedKit);
  t.diagnostic(JSON.stringify({ agentInterface: document }));
}

async function agentConnectSectionUsesPublicConfigurationAndNoKey(
  t: TestContext,
) {
  const document = await readInterface();
  const { Window } = await import("happy-dom");
  const dom = new Window({ url: origin });
  const originalGlobals = new Map<string, PropertyDescriptor | undefined>();
  let cleanup: (() => void) | undefined;
  try {
    for (const [name, value] of Object.entries({
      window: dom,
      document: dom.document,
      navigator: dom.navigator,
      HTMLElement: dom.HTMLElement,
      HTMLTextAreaElement: dom.HTMLTextAreaElement,
      Element: dom.Element,
      Node: dom.Node,
      Event: dom.Event,
      MutationObserver: dom.MutationObserver,
      ResizeObserver: dom.ResizeObserver,
      getComputedStyle: dom.getComputedStyle.bind(dom),
      requestAnimationFrame: dom.requestAnimationFrame.bind(dom),
      cancelAnimationFrame: dom.cancelAnimationFrame.bind(dom),
      fetch: dom.fetch.bind(dom),
      IS_REACT_ACT_ENVIRONMENT: true,
    })) {
      originalGlobals.set(
        name,
        Object.getOwnPropertyDescriptor(globalThis, name),
      );
      Object.defineProperty(globalThis, name, {
        value,
        configurable: true,
        writable: true,
      });
    }
    const React = await import("react");
    originalGlobals.set(
      "React",
      Object.getOwnPropertyDescriptor(globalThis, "React"),
    );
    Object.assign(globalThis, { React });
    const testing = await import("@testing-library/react");
    cleanup = testing.cleanup;
    const { AgentConnect } =
      await import("../packages/portfolio/src/pages/agents/agents.connect.ui");
    const { AGENT_KEY_FILE_PLACEHOLDER, agentMcpConfiguration } =
      await import("../packages/portfolio/src/pages/agents/agents.connect");
    const { gatewayAgentDid } =
      await import("../packages/agent-kit/src/gateway");
    const did = gatewayAgentDid(Buffer.from(seed, "hex"), "connect");
    const view = testing.render(React.createElement(AgentConnect, { did }));
    const textarea = (await view.findByLabelText(
      "MCP client configuration",
    )) as HTMLTextAreaElement;
    assert.equal(textarea.readOnly, true);
    const config = JSON.parse(textarea.value);
    assert.deepEqual(config, agentMcpConfiguration(document, did));
    const server = config.mcpServers["paxeer-perps"];
    assert.deepEqual(
      Object.keys(server.env),
      document.environment.map((entry) => entry.name),
    );
    assert.equal(
      server.env[AGENT_ENV.walletGateway],
      document.connection.walletGatewayUrl,
    );
    assert.equal(server.env[AGENT_ENV.node], document.connection.nodeUrl);
    assert.equal(
      server.env[AGENT_ENV.dataGateway],
      document.connection.dataGatewayUrl,
    );
    assert.equal(server.env[AGENT_ENV.keyFile], AGENT_KEY_FILE_PLACEHOLDER);
    assert.equal(server.env[AGENT_ENV.label], "connect");
    assert.equal(server.command, document.entry.command);
    assert.deepEqual(server.args, document.entry.args);
    assert.equal(server.cwd, "apps/perps");
    assert.equal(
      view
        .getByRole("link", { name: "/agents/interface.json" })
        .getAttribute("href"),
      "/agents/interface.json",
    );
    const section = view.getByRole("region", { name: "Connect this agent" });
    assert.match(section.textContent!, /Selected agent:/);
    assert.ok(section.textContent!.includes(did));
    assert.match(section.textContent!, /Model Context Protocol \(MCP\)/);
    assert.match(
      section.textContent!,
      /Keep the agent key on the machine running your client\./,
    );
    const steps = view
      .getAllByRole("listitem")
      .map((item) => item.textContent!.replace(/\s+/g, " "));
    assert.deepEqual(steps, [
      "Save this agent's 32-byte Ed25519 seed as 64 hex characters in an owner-only file on the client machine. Replace the key-file placeholder in the configuration with that file's path. Use the key that belongs to the selected DID; keep its label unchanged.",
      "Install the workspace dependencies with pnpm --dir apps/perps install --frozen-lockfile. Add the configuration to your MCP client and set its working directory to apps/perps in your checkout. Review this agent's limits, fund USDL and PAX for gas, and unfreeze it when ready to trade.",
      "Start the MCP server, list tools, and call markets with {}. Read account before depositing collateral or placing an order.",
    ]);
    assert.equal(section.querySelectorAll("input").length, 0);
    assert.equal(
      section.querySelectorAll("textarea:not([readonly])").length,
      0,
    );
    assert.ok(!section.outerHTML.includes(seed));
    assert.ok(!textarea.value.includes(seed));
    assert.ok(!textarea.value.includes(keyFile));
    assert.doesNotMatch(
      textarea.value,
      /Bearer |access_token|privateKey|supabase/i,
    );
    testing.fireEvent.click(
      view.getByRole("button", { name: "Copy MCP configuration" }),
    );
    await view.findByText("MCP configuration copied.");
    assert.equal(await dom.navigator.clipboard.readText(), textarea.value);
    t.diagnostic(
      JSON.stringify({
        agentConnect: { text: section.textContent, configuration: config },
      }),
    );
    const otherDid = gatewayAgentDid(randomBytes(32), "other");
    view.rerender(React.createElement(AgentConnect, { did: otherDid }));
    await testing.waitFor(() =>
      assert.equal(
        JSON.parse(
          (
            view.getByLabelText(
              "MCP client configuration",
            ) as HTMLTextAreaElement
          ).value,
        ).mcpServers["paxeer-perps"].env[AGENT_ENV.label],
        "other",
      ),
    );
    assert.ok(view.getByText(`Selected agent: ${otherDid}`));
    const source = await readFile(
      path.join(root, "packages/portfolio/src/pages/agents/agents.ui.tsx"),
      "utf8",
    );
    assert.match(
      source,
      /detail.status === "ready" && <AgentConnect key=\{did\} did=\{did\}/,
    );
  } finally {
    cleanup?.();
    await dom.happyDOM.close();
    for (const [name, descriptor] of originalGlobals) {
      if (descriptor) Object.defineProperty(globalThis, name, descriptor);
      else Reflect.deleteProperty(globalThis, name);
    }
  }
}

async function agentInterfaceRejectsCredentialBearingEndpoints() {
  const { agentInterfaceDocument } =
    await import("../apps/web/src/server/agents");
  const configured = process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
  try {
    for (const raw of [
      `${origin}/wallet?token=${seed}`,
      `${origin}/wallet#${seed}`,
      origin.replace("//", `//user:${seed}@`),
      "file:///wallet",
      "",
    ]) {
      process.env.NEXT_PUBLIC_PAXEER_WALLET_API = raw;
      assert.throws(() => agentInterfaceDocument());
    }
  } finally {
    if (configured === undefined)
      delete process.env.NEXT_PUBLIC_PAXEER_WALLET_API;
    else process.env.NEXT_PUBLIC_PAXEER_WALLET_API = configured;
  }
}

test({ timeout: 120_000 }, agentInterfaceDocumentMatchesTheKit);
test({ timeout: 30_000 }, agentConnectSectionUsesPublicConfigurationAndNoKey);
test(agentInterfaceRejectsCredentialBearingEndpoints);
