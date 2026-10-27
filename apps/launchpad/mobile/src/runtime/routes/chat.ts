import {
  decodeContract,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export function conversations(sdk: SdkClient, signal: AbortSignal) {
  return sdk.request('/v1/me/conversations?limit=50', 'DmListResponse', {
    signal,
    authenticated: true,
    idempotentRead: true,
  });
}

export function messages(sdk: SdkClient, room: string, signal: AbortSignal) {
  return sdk.request(
    `/v1/rooms/${identifier(room)}/messages?limit=50`,
    'MessagesListResponse',
    { signal, authenticated: true, idempotentRead: true },
  );
}

export function sendMessage(
  sdk: SdkClient,
  room: string,
  body: string,
  clientMessageId: string,
  signal: AbortSignal,
) {
  return sdk.request(
    `/v1/rooms/${identifier(room)}/messages`,
    'MessageSendResponse',
    {
      signal,
      method: 'POST',
      authenticated: true,
      body: decodeContract('MessageSendBody', { body, clientMessageId }),
    },
  );
}

function identifier(value: string) {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(value))
    throw new TypeError('Invalid resource identifier');
  return value;
}
