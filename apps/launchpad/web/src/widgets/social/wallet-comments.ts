import { socialReadUrl } from '@/core/sdk-config';
import { InteractionClient, type Comment } from '@/widgets/community-interactions/interaction-client';

export async function readWalletComments(wallet: string, signal?: AbortSignal) {
  const client = new InteractionClient(socialReadUrl('/v2/social'));
  const query = new URLSearchParams({ limit: '50' });
  return client['request']<{ items: Comment[] }>(`/users/${encodeURIComponent(wallet)}/comments?${query}`, 'GET', undefined, undefined, signal);
}
