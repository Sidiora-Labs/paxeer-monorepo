import { createServer, type IncomingHttpHeaders } from 'node:http';
import { expect, test } from 'vitest';
import {
  SdkClient,
  type V2ContractTypes,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import { chatRoutes } from '../../src/runtime/routes/chat.ts';

const roomId = '123e4567-e89b-42d3-a456-426614174001';
const clientMessageId = '123e4567-e89b-42d3-a456-426614174002';
const peer = `0x${'ab'.repeat(20)}`;
const time = '2026-01-01T00:00:00.000Z';
const conversationResponse = {
  conversations: [
    {
      id: roomId,
      peer,
      last_message_at: 1767225600,
      unread_count: 2,
      read_sequence: '3',
      muted: false,
    },
    {
      id: '123e4567-e89b-42d3-a456-426614174003',
      peer: `0x${'cd'.repeat(20)}`,
      last_message_at: null,
      unread_count: 0,
      read_sequence: '0',
      muted: true,
    },
  ],
} satisfies V2ContractTypes['ConnectedChatConversationsResponse'];
const message = {
  authorWallet: peer,
  body: 'Hello, served chat! 🌍',
  createdAt: time,
  deletedAt: null,
  editedAt: null,
  messageId: '123e4567-e89b-72d3-a456-426614174005',
  revision: '1',
  roomId,
} satisfies V2ContractTypes['Message'];
const envelope = {
  generatedAt: time,
  nextCursor: null,
  quality: 'complete',
  requestId: '123e4567-e89b-72d3-a456-426614174006',
  revision: '1',
  watermark: null,
} as const;
const messagesResponse = {
  ...envelope,
  data: {
    items: [
      message,
      {
        ...message,
        messageId: '123e4567-e89b-72d3-a456-426614174004',
        body: '',
        deletedAt: time,
        editedAt: time,
      },
    ],
  },
} satisfies V2ContractTypes['MessagesListResponse'];
const sendResponse = {
  ...envelope,
  data: { message },
} satisfies V2ContractTypes['MessageSendResponse'];

async function chatServer() {
  const requests: {
    method: string | undefined;
    path: string | undefined;
    headers: IncomingHttpHeaders;
    body: string;
  }[] = [];
  const server = createServer((request, response) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk: string) => {
      body += chunk;
    });
    request.on('end', () => {
      requests.push({
        method: request.method,
        path: request.url,
        headers: request.headers,
        body,
      });
      const payload =
        request.method === 'GET' && request.url === '/v2/chat/conversations'
          ? conversationResponse
          : request.method === 'GET' &&
              request.url === `/v2/rooms/${roomId}/messages?limit=50`
            ? messagesResponse
            : request.method === 'POST' &&
                request.url === `/v2/rooms/${roomId}/messages`
              ? sendResponse
              : undefined;
      response.writeHead(payload ? 200 : 404, {
        'Content-Type': 'application/json',
      });
      response.end(JSON.stringify(payload ?? { error: 'Unknown route' }));
    });
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const bound = server.address();
  if (!bound || typeof bound === 'string')
    throw new Error('Loopback port missing');
  const origin = `http://127.0.0.1:${bound.port}`;
  const streamOrigin = `ws://127.0.0.1:${bound.port}`;
  const sdk = new SdkClient({
    apiOrigin: origin,
    identityOrigin: origin,
    mediaOrigin: origin,
    streamOrigin,
    approvedStreamOrigins: [streamOrigin],
    production: false,
    session: {
      mode: 'cookie',
      applicationOrigin: origin,
      csrfToken: () => 'chat-csrf',
    },
  });
  return {
    sdk,
    requests,
    close: () =>
      new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
        server.closeAllConnections();
      }),
  };
}

async function chatReadsUseServedRoutes() {
  const server = await chatServer();
  try {
    const signal = new AbortController().signal;
    expect(await chatRoutes.conversations(server.sdk, signal)).toEqual({
      data: {
        items: conversationResponse.conversations.map((conversation) => ({
          conversationId: conversation.id,
          peer: conversation.peer,
          lastMessageAt: conversation.last_message_at,
          unreadCount: conversation.unread_count,
          readSequence: conversation.read_sequence,
          muted: conversation.muted,
        })),
      },
    });
    expect(await chatRoutes.messages(server.sdk, roomId, signal)).toEqual(
      messagesResponse,
    );
    expect(
      server.requests.map(({ method, path, body, headers }) => ({
        method,
        path,
        body,
        csrf: headers['x-csrf-token'],
      })),
    ).toEqual([
      {
        method: 'GET',
        path: '/v2/chat/conversations',
        body: '',
        csrf: undefined,
      },
      {
        method: 'GET',
        path: `/v2/rooms/${roomId}/messages?limit=50`,
        body: '',
        csrf: undefined,
      },
    ]);
    await expect(
      chatRoutes.messages(server.sdk, '../invalid', signal),
    ).rejects.toThrow('Invalid IF_MESSAGES_LIST path');
    await expect(
      chatRoutes.conversations(server.sdk, AbortSignal.abort()),
    ).rejects.toThrow();
    await expect(
      chatRoutes.messages(server.sdk, roomId, AbortSignal.abort()),
    ).rejects.toThrow();
    expect(server.requests).toHaveLength(2);
  } finally {
    await server.close();
  }
}

async function chatSendUsesRoomMessagesRoute() {
  const server = await chatServer();
  try {
    const signal = new AbortController().signal;
    expect(
      await chatRoutes.sendMessage(
        server.sdk,
        roomId,
        message.body,
        clientMessageId,
        signal,
      ),
    ).toEqual(sendResponse);
    expect(server.requests).toHaveLength(1);
    const request = server.requests[0];
    expect(request.method).toBe('POST');
    expect(request.path).toBe(`/v2/rooms/${roomId}/messages`);
    expect(JSON.parse(request.body)).toEqual({
      body: message.body,
      clientMessageId,
    });
    expect(request.headers['content-type']).toBe('application/json');
    expect(request.headers['x-csrf-token']).toBe('chat-csrf');
    expect(request.headers['idempotency-key']).toBe(clientMessageId);
    await expect(
      chatRoutes.sendMessage(
        server.sdk,
        '../invalid',
        message.body,
        clientMessageId,
        signal,
      ),
    ).rejects.toThrow('Invalid IF_MESSAGE_SEND path');
    await expect(
      chatRoutes.sendMessage(
        server.sdk,
        roomId,
        message.body,
        'invalid-client-message-id',
        signal,
      ),
    ).rejects.toThrow('Invalid v2 contract MessageSendBody');
    await expect(
      chatRoutes.sendMessage(
        server.sdk,
        roomId,
        message.body,
        clientMessageId,
        AbortSignal.abort(),
      ),
    ).rejects.toThrow();
    expect(server.requests).toHaveLength(1);
  } finally {
    await server.close();
  }
}

test(chatReadsUseServedRoutes, chatReadsUseServedRoutes);
test(chatSendUsesRoomMessagesRoute, chatSendUsesRoomMessagesRoute);
