#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { httpUrl } from "./gateway.js";
import { jsonText } from "./result.js";

function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error("Missing agent configuration");
  return value;
}

async function main(): Promise<void> {
  const gatewayUrl = httpUrl(required("PAXEER_AGENT_GATEWAY_URL"));
  const nodeUrl = httpUrl(required("PAXEER_AGENT_NODE_URL"));
  const dataUrl = httpUrl(required("NEXT_PUBLIC_PERPS_GATEWAY_URL"));
  if (new URL(dataUrl).pathname !== "/") throw new Error("Invalid data origin");
  const seed = (
    await readFile(required("PAXEER_AGENT_KEY_FILE"), "utf8")
  ).trim();
  if (!/^[0-9a-fA-F]{64}$/.test(seed))
    throw new Error("Invalid agent key file");
  const { AgentGatewayClient, PerpsDataClient, createAgentTools } =
    await import("./index.js");
  const gateway = new AgentGatewayClient({
    url: gatewayUrl,
    privateKey: Buffer.from(seed, "hex"),
    label: process.env.PAXEER_AGENT_LABEL ?? "perps",
  });
  const kit = createAgentTools(
    gateway,
    new PerpsDataClient({ nodeUrl, dataUrl }),
  );
  const server = new McpServer({
    name: "paxeer-perps-agent-kit",
    version: "0.1.0",
  });
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
