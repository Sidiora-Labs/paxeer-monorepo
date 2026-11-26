'use client';

import { useState } from 'react';
import { Bot } from 'lucide-react';
import { Button, EmptyState, FormField, Panel } from '@/ui/primitives';
import { AGENT_INTERFACE_PATH, agentClientConfiguration, type AgentConnection } from './agent-connection';

export type AgentCopyState = 'idle' | 'copying' | 'copied' | 'manual';
export interface AgentConnectProps {
  agent: { did: string; label: string } | null;
  connection: AgentConnection;
}

export function AgentConnectView({ agent, connection, copyState, onCopy }: AgentConnectProps & { copyState: AgentCopyState; onCopy: () => void }) {
  return <Panel aria-labelledby="agent-connect-title" className="flex min-w-0 flex-col gap-3">
    <h2 id="agent-connect-title" className="text-md font-semibold">Connect your agent</h2>
    <a href={AGENT_INTERFACE_PATH} className="min-h-11 break-all py-2 text-sm underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">{AGENT_INTERFACE_PATH}</a>
    {!agent ? <EmptyState icon={Bot} title="Choose an agent to connect" body="Create or claim an agent, then select it in Your agents to see its connection instructions." /> : connection.status === 'unavailable' ? <EmptyState icon={Bot} title="Connection details unavailable" body="The application needs valid wallet gateway and data addresses. Reload after its configuration is available." action={<Button onClick={() => window.location.reload()}>Reload connection details</Button>} /> : <>
      <p className="break-all text-sm text-text-muted">Connect {agent.label}: {agent.did}</p>
      <ol className="flex list-decimal flex-col gap-2 pl-5 text-sm text-text-muted">
        <li>On the agent’s machine, save its Ed25519 seed in a private JSON file as seedHex, with label set to <code className="break-all">{agent.did.split(':')[2]}</code>. Set file permissions to 0600. Keep the file on that machine.</li>
        <li>Build the agent kit and start your Model Context Protocol (MCP) client from the repository root. Add the configuration below, replace &lt;AGENT_KEY_FILE&gt; with the file’s absolute path, and restart the client.</li>
        <li>Call <code>agent_identity</code> with <code>{'{}'}</code> and confirm its DID matches this agent. Then call <code>list_coins</code> with <code>{'{"limit":1}'}</code>. Fund the agent wallet and review its limits before trading.</li>
      </ol>
      <FormField id="agent-client-configuration" label="MCP client configuration" multiline mono textareaProps={{ value: agentClientConfiguration(connection), readOnly: true, rows: 15, spellCheck: false, autoComplete: 'off', wrap: 'soft' }} />
      <Button disabled={copyState === 'copying'} onClick={onCopy}>{copyState === 'copying' ? 'Copying configuration' : 'Copy configuration'}</Button>
      <p role="status" aria-live="polite" className="text-sm text-text-muted">{copyState === 'copied' ? 'Configuration copied.' : copyState === 'manual' ? 'Copy is unavailable. Select the configuration and copy it manually.' : 'The configuration contains public addresses and a file path placeholder. Your key stays on the agent’s machine.'}</p>
    </>}
  </Panel>;
}

export function AgentConnect(props: AgentConnectProps) {
  const [copyState, setCopyState] = useState<AgentCopyState>('idle');
  async function copy() {
    if (props.connection.status !== 'ready' || !props.agent || copyState === 'copying') return;
    setCopyState('copying');
    try {
      await navigator.clipboard.writeText(agentClientConfiguration(props.connection));
      setCopyState('copied');
    } catch {
      setCopyState('manual');
    }
  }
  return <AgentConnectView {...props} copyState={copyState} onCopy={() => void copy()} />;
}
