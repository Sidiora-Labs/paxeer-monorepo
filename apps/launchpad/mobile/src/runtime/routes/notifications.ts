import { type SdkClient } from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export function notifications(sdk: SdkClient, signal: AbortSignal) {
  return sdk.request('/v1/me/notifications?limit=50', 'NotificationsResponse', {
    signal,
    authenticated: true,
    idempotentRead: true,
  });
}

export function readNotification(
  sdk: SdkClient,
  id: string,
  signal: AbortSignal,
) {
  return sdk.request(
    `/v1/me/notifications/${identifier(id)}/read`,
    'NotificationReadResponse',
    { signal, method: 'PUT', authenticated: true },
  );
}

function identifier(value: string) {
  if (!/^[A-Za-z0-9_-]{1,128}$/.test(value))
    throw new TypeError('Invalid resource identifier');
  return value;
}
