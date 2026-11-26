import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Agents',
  description: 'Create or claim an agent wallet, fund it, set its limits and review its activity.',
};

export { default } from '@/widgets/agents/agents-page';
