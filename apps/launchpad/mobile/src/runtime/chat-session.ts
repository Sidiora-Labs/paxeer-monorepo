import { directMessageFrame } from './chat-frame.ts';
import { sha256, stringToHex } from 'viem';
import {
  Operations,
  decodeV2Contract,
  type SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export type DurableMessage =
  V2ContractTypes['ConnectedFanoutDurableMessage'];
export type MessageHistory =
  V2ContractTypes['ConnectedFanoutHistoryPage'];

export async function conversationHistory(
  sdk: SdkClient,
  room: string,
  signal: AbortSignal,
  cursor?: string,
) {
  const page = await new Operations(sdk).call(
    'IF_CONNECTED_CHAT_HISTORY',
    {
      query: {
        conversation_id: room,
        limit: 50,
        ...(cursor ? { cursor } : {}),
      },
    },
    { signal },
  );
  if (page.reset_required)
    throw new Error('Conversation changed. Return to the first page.');
  for (const message of page.messages) {
    if (message.conversation_id !== room)
      throw new Error('Conversation history mismatch');
  }
  return page;
}

export function durableAcknowledgement(
  value: unknown,
  room: string,
  clientMessageId: string,
) {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    throw new Error('Invalid chat response');
  const frame = value as Record<string, unknown>;
  if (
    frame.type !== 'message_persisted' ||
    frame.client_message_id !== clientMessageId
  )
    return undefined;
  if (
    frame.conversation_id !== room ||
    typeof frame.id !== 'string' ||
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      frame.id,
    ) ||
    typeof frame.sequence !== 'string' ||
    !/^[1-9][0-9]{0,18}$/.test(frame.sequence) ||
    typeof frame.revision !== 'string' ||
    !/^[1-9][0-9]{0,18}$/.test(frame.revision)
  )
    throw new Error('Chat acknowledgement identity mismatch');
  if (
    BigInt(frame.sequence) > 9223372036854775807n ||
    BigInt(frame.revision) > 9223372036854775807n
  )
    throw new Error(
      'Chat acknowledgement sequence exceeds the supported range',
    );
  return {
    messageId: frame.id,
    sequence: frame.sequence,
    revision: frame.revision,
  };
}

export async function sendDirectMessage(
  sdk: SdkClient,
  wallet: string,
  room: string,
  peer: string,
  body: string,
  clientMessageId: string,
  signal: AbortSignal,
) {
  const outgoing = directMessageFrame(peer, body, clientMessageId);
  if (
    !/^0x[0-9a-f]{40}$/i.test(wallet) ||
    !/^0x[0-9a-f]{40}$/i.test(peer)
  )
    throw new Error('Current conversation wallets required');
  const topics = [`private.chat.${wallet.toLowerCase()}`];
  const issued = await sdk.requestV2(
    '/v2/stream/tickets',
    'StreamTicketIssueResponse',
    {
      method: 'POST',
      service: 'identity',
      authenticated: true,
      signal,
      body: decodeV2Contract('StreamTicketIssueBody', {
        schemaMajor: 2,
        topics,
        filterDigest: sha256(stringToHex(JSON.stringify(topics))).slice(
          2,
        ),
        origin:
          sdk.config.session?.mode === 'cookie'
            ? sdk.config.session.applicationOrigin
            : sdk.config.apiOrigin,
        endpoint: '/v2/stream',
      }),
    },
  );
  const ticket = issued.data;
  if (Date.parse(ticket.expiresAt) <= Date.now())
    throw new Error('Chat authorization expired');
  signal.throwIfAborted();
  const endpoint = new URL('/v2/stream', sdk.config.streamOrigin);
  endpoint.searchParams.set('topics', topics.join(','));
  return new Promise<
    NonNullable<ReturnType<typeof durableAcknowledgement>>
  >((resolve, reject) => {
    const socket: WebSocket = Reflect.construct(WebSocket, [
      endpoint.toString(),
      ['sidiora.v2', `ticket.${ticket.ticket}`],
      {
        headers: {
          Origin:
            sdk.config.session?.mode === 'cookie'
              ? sdk.config.session.applicationOrigin
              : sdk.config.apiOrigin,
        },
      },
    ]);
    let accepted = false,
      sent = false,
      settled = false;
    const finish = (
      error?: Error,
      result?: NonNullable<ReturnType<typeof durableAcknowledgement>>,
    ) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal.removeEventListener('abort', abort);
      socket.onopen =
        socket.onmessage =
        socket.onerror =
        socket.onclose =
          null;
      try {
        socket.close();
      } catch {
        /* The socket can close before its opening handshake. */
      }
      if (error) reject(error);
      else resolve(result!);
    };
    const abort = () =>
      finish(
        new Error(
          'Message outcome unknown. Retry the same message to reconcile.',
        ),
      );
    const timer = setTimeout(abort, 15000);
    signal.addEventListener('abort', abort, { once: true });
    if (signal.aborted) {
      abort();
      return;
    }
    socket.onopen = () => {
      if (socket.protocol !== 'sidiora.v2')
        finish(new Error('Chat protocol unavailable'));
    };
    socket.onmessage = (event) => {
      try {
        const value: unknown = JSON.parse(String(event.data));
        if (!value || typeof value !== 'object' || !('type' in value))
          throw new Error('Invalid chat response');
        const frame = value as Record<string, unknown>;
        if (!accepted) {
          if (
            frame.type !== 'session_accepted' ||
            frame.protocol_version !== 2
          )
            throw new Error('Chat session was not accepted');
          accepted = true;
          socket.send(JSON.stringify({ type: 'subscribe_dms' }));
          return;
        }
        if (frame.type === 'subscribed_dms' && !sent) {
          sent = true;
          socket.send(JSON.stringify(outgoing));
          return;
        }
        const result = durableAcknowledgement(
          frame,
          room,
          clientMessageId,
        );
        if (result) {
          finish(undefined, result);
          return;
        }
        if (
          frame.type === 'error' ||
          frame.type === 'message_uncertain'
        ) {
          if (
            frame.client_message_id !== undefined &&
            frame.client_message_id !== clientMessageId
          )
            return;
          throw new Error(
            frame.type === 'message_uncertain'
              ? 'Message outcome unknown. Retry the same message to reconcile.'
              : 'Chat refused this message. Refresh the conversation before retrying.',
          );
        }
      } catch (error) {
        finish(
          error instanceof Error
            ? error
            : new Error('Invalid chat response'),
        );
      }
    };
    socket.onerror = abort;
    socket.onclose = abort;
  });
}
