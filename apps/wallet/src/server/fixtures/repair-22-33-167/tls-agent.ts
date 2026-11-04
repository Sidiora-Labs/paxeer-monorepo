import { Agent, type AgentOptions } from 'node:https';

// Keep provider URL validation intact while connecting to the committed TLS peer.
export function loopbackAgent(port: number, options: AgentOptions): Agent {
  const agent = new Agent({ ...options, servername: 'localhost' });
  const connect = agent.createConnection.bind(agent);
  agent.createConnection = (connectionOptions, callback) =>
    connect({ ...connectionOptions, host: '127.0.0.1', port }, callback);
  return agent;
}
