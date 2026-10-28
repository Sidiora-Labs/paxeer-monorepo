import {
  Operations,
  type SdkClient,
} from '../../../../../../sdk/launchpad/packages/sdk/src/index.ts';

export const IF_CONNECTED_NOTIFICATION_READ = 'IF_CONNECTED_NOTIFICATION_READ';

export async function notifications(sdk: SdkClient, signal: AbortSignal) {
  const items = await new Operations(sdk).call(
    'IF_CONNECTED_NOTIFICATION_LIST',
    {},
    { signal },
  );
  return items.map((item) => ({
    notificationId: item.ID,
    category: item.Category,
    summary: item.Summary,
    resourceId: item.ResourceID,
    createdAt: item.CreatedAt,
    expiresAt: item.ExpiresAt,
    read: item.Read,
  }));
}

export function readNotification(
  sdk: SdkClient,
  id: string,
  signal: AbortSignal,
) {
  return new Operations(sdk).call(
    IF_CONNECTED_NOTIFICATION_READ,
    { path: { id: identifier(id) }, body: { read: true } },
    { signal },
  );
}

export const notificationRoutes = { notifications, readNotification };

function identifier(value: string) {
  if (!/^[0-9a-f]{64}$/.test(value))
    throw new TypeError('Invalid resource identifier');
  return value;
}
