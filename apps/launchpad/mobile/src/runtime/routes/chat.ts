import {
  Operations,
  type SdkClient,
  type V2ContractTypes,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export const IF_MESSAGE_SEND = 'IF_MESSAGE_SEND';

export type ChatConversation = {
  conversationId: string;
  peer: string;
  unreadCount: number;
  muted: boolean;
  lastMessageAt: number | null;
  readSequence: string;
};
export type ChatMessage = V2ContractTypes['Message'];

export async function conversations(sdk: SdkClient, signal: AbortSignal) {
  const response = await new Operations(sdk).call(
    'IF_CONNECTED_CHAT_CONVERSATIONS',
    {},
    { signal },
  );
  return {
    data: {
      items: response.conversations.map(
        (conversation): ChatConversation => ({
          conversationId: conversation.id,
          peer: conversation.peer,
          unreadCount: conversation.unread_count,
          muted: conversation.muted,
          lastMessageAt: conversation.last_message_at,
          readSequence: conversation.read_sequence,
        }),
      ),
    },
  };
}

export function messages(sdk: SdkClient, room: string, signal: AbortSignal) {
  return new Operations(sdk).call(
    'IF_MESSAGES_LIST',
    { path: { roomId: room }, query: { limit: 50 } },
    { signal },
  );
}

export function sendMessage(
  sdk: SdkClient,
  room: string,
  body: string,
  clientMessageId: string,
  signal: AbortSignal,
) {
  return new Operations(sdk).call(
    IF_MESSAGE_SEND,
    { path: { roomId: room }, body: { body, clientMessageId } },
    { signal, headers: { 'Idempotency-Key': clientMessageId } },
  );
}

export const chatRoutes = { conversations, messages, sendMessage };
