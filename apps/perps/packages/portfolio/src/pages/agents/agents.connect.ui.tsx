import React, { useEffect, useState } from "react";
import { Button, Flex, Text } from "@paxeer-network/ui";
import type { AgentWorkspaceInterface } from "@paxeer-network/agent-kit/interface";
import { AGENT_INTERFACE_PATH, agentMcpConfiguration } from "./agents.connect";

type Connection =
  | { status: "loading" }
  | { status: "ready"; text: string }
  | { status: "error" };

export function AgentConnect({ did }: { did: string }) {
  const [connection, setConnection] = useState<Connection>({
    status: "loading",
  });
  const [revision, setRevision] = useState(0);
  const [notice, setNotice] = useState<string | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setConnection({ status: "loading" });
    setNotice(null);
    void (async () => {
      try {
        const response = await fetch(AGENT_INTERFACE_PATH, {
          credentials: "omit",
          signal: controller.signal,
        });
        if (!response.ok) throw new Error("Agent interface is unavailable");
        const document: AgentWorkspaceInterface = await response.json();
        const text = JSON.stringify(
          agentMcpConfiguration(document, did),
          null,
          2,
        );
        if (!controller.signal.aborted)
          setConnection({ status: "ready", text });
      } catch {
        if (!controller.signal.aborted) setConnection({ status: "error" });
      }
    })();
    return () => controller.abort();
  }, [did, revision]);
  return (
    <section
      aria-labelledby="agent-connect-heading"
      className="oui-flex oui-flex-col oui-gap-4 oui-min-w-0"
    >
      <h3 id="agent-connect-heading">Connect this agent</h3>
      <Text as="p" size="xs" className="oui-break-all">
        Selected agent: {did}
      </Text>
      <Text as="p" size="sm" intensity={54}>
        Connect a Model Context Protocol (MCP) client to the trading workspace.
        The configuration contains public addresses and a key-file placeholder.
        Keep the agent key on the machine running your client.
      </Text>
      <Text as="p" size="xs">
        Agent interface document:{" "}
        <a
          className="oui-text-primary-light oui-break-all"
          href={AGENT_INTERFACE_PATH}
        >
          {AGENT_INTERFACE_PATH}
        </a>
      </Text>
      <ol className="oui-flex oui-flex-col oui-gap-4 oui-text-xs oui-list-decimal oui-pl-5">
        <li>
          Save this agent's 32-byte Ed25519 seed as 64 hex characters in an
          owner-only file on the client machine. Replace the key-file
          placeholder in the configuration with that file's path. Use the key
          that belongs to the selected DID; keep its label unchanged.
        </li>
        <li>
          Install the workspace dependencies with pnpm --dir apps/perps install
          --frozen-lockfile. Add the configuration to your MCP client and set
          its working directory to apps/perps in your checkout. Review this
          agent's limits, fund USDL and PAX for gas, and unfreeze it when ready
          to trade.
        </li>
        <li>
          Start the MCP server, list tools, and call markets with {"{}"}. Read
          account before depositing collateral or placing an order.
        </li>
      </ol>
      {connection.status === "loading" && (
        <p role="status">Loading agent connection…</p>
      )}
      {connection.status === "error" && (
        <div role="alert">
          <p>Unable to load the agent interface document.</p>
          <Button
            size="md"
            variant="outlined"
            onClick={() => setRevision((value) => value + 1)}
          >
            Retry connection
          </Button>
        </div>
      )}
      {connection.status === "ready" && (
        <>
          <label className="oui-flex oui-flex-col oui-gap-4 oui-min-w-0">
            MCP client configuration
            <textarea
              className="oui-w-full oui-rounded oui-bg-base-6 oui-p-3 oui-text-xs oui-text-base-contrast focus-visible:oui-outline-primary-light"
              readOnly
              rows={15}
              value={connection.text}
              spellCheck={false}
              autoComplete="off"
            />
          </label>
          <Flex gap={2} wrap="wrap">
            <Button
              size="md"
              variant="outlined"
              onClick={() => {
                void (async () => {
                  try {
                    await navigator.clipboard.writeText(connection.text);
                    setNotice("MCP configuration copied.");
                  } catch {
                    setNotice(
                      "Select the configuration text and copy it manually.",
                    );
                  }
                })();
              }}
            >
              Copy MCP configuration
            </Button>
          </Flex>
        </>
      )}
      {notice && <p role="status">{notice}</p>}
    </section>
  );
}
