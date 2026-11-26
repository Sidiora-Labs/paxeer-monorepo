#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { httpUrl } from "./gateway.js";
import { jsonText } from "./result.js";
import { AGENT_ENV, describeAgentInterface } from "./interface.js";

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error("Missing agent configuration");
  return value;
}

async function main(): Promise<void> {
  const gatewayUrl = httpUrl(required(AGENT_ENV.walletGateway));
  const nodeUrl = httpUrl(required(AGENT_ENV.node));
  const dataUrl = httpUrl(required(AGENT_ENV.dataGateway));
  if (new URL(dataUrl).pathname !== "/") throw new Error("Invalid data origin");
  const seed = (await readFile(required(AGENT_ENV.keyFile), "utf8")).trim();
  if (!/^[0-9a-fA-F]{64}$/.test(seed))
    throw new Error("Invalid agent key file");
  const { AgentGatewayClient, PerpsDataClient, createAgentTools } =
    await import("./index.js");
  const gateway = new AgentGatewayClient({
    url: gatewayUrl,
    privateKey: Buffer.from(seed, "hex"),
    label: process.env[AGENT_ENV.label] ?? "perps",
  });
  const kit = createAgentTools(
    gateway,
    new PerpsDataClient({ nodeUrl, dataUrl }),
  );
  const { name, version } = describeAgentInterface();
  const server = new McpServer({ name, version });
  for (const entry of kit.tools) {
    server.registerTool(
      entry.name,
      {
        description: entry.description,
        inputSchema: entry.inputSchema,
        annotations: {
          readOnlyHint: entry.readOnly,
          destructiveHint: !entry.readOnly,
          idempotentHint: entry.readOnly,
          openWorldHint: true,
        },
      },
      async (input) => {
        const result = await entry.execute(input);
        return {
          isError: !result.ok,
          content: [{ type: "text" as const, text: jsonText(result) }],
        };
      },
    );
  }
  await server.connect(new StdioServerTransport());
}

main().catch(() => {
  process.stderr.write(
    "Unable to start the perps agent server; check its environment and Ed25519 key file.\n",
  );
  process.exitCode = 1;
});
