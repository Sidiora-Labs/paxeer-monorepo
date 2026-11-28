import type { Metadata } from 'next';
import AgentsPage from '@/widgets/agents/agents-page';
import { agentConnectionConfiguration } from '@/core/agent-interface';

export const metadata: Metadata = {
  title: 'Agents',
  description: 'Create or claim an agent wallet, fund it, set its limits and review its activity.',
};

export default function Page() {
  return <AgentsPage connection={agentConnectionConfiguration()} />;
}
