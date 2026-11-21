import { decodeV2Contract } from '@kindle/sdk';

export function directMessageFrame(
  peer: string,
  body: string,
  clientMessageId: string,
) {
  decodeV2Contract('MessageSendBody', { body, clientMessageId });
  if (!/^0x[0-9a-f]{40}$/i.test(peer))
    throw new Error('Current conversation wallet required');
  if (!body.trim() || [...body].length > 500)
    throw new Error('Enter a message of at most 500 characters.');
  return {
    type: 'dm' as const,
    to: peer.toLowerCase(),
    content: body,
    client_message_id: clientMessageId,
  };
}
