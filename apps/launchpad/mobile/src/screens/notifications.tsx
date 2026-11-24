import React, { useState } from 'react';
import {
  Button,
  Text,
  Card,
  ErrorNotice,
  ListPage,
  ResourceStatus,
  showAction,
  useResource,
  useSurface,
  type Runtime,
} from '../navigation.tsx';
export function Notifications({ runtime }: { runtime: Runtime }) {
  const { ui } = useSurface();
  const inbox = useResource(
      runtime,
      'discovery-notifications',
      (s) => runtime.services.notifications(s),
      true,
    ),
    preferences = useResource(
      runtime,
      'preferences',
      (s) => runtime.services.preferences(s),
      true,
    );
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  return (
    <ListPage
      runtime={runtime}
      stateKey="private:notifications"
      title="Notifications"
      data={(inbox.data ?? []).filter(
        (item, index, items) =>
          items.findIndex(
            (other) => other.notificationId === item.notificationId,
          ) === index,
      )}
      keyOf={(item) => item.notificationId}
      header={
        <>
          <ResourceStatus {...inbox} />
          <ResourceStatus {...preferences} />
          {error && <ErrorNotice message={error} />}
          {notice && <Text style={ui.text}>{notice}</Text>}
          <Text style={ui.subtle}>
            Optional push notifications show generic lock-screen text. Tap
            opens current authorized inbox content.
          </Text>
          {runtime.push?.tokenChanged && (
            <Text style={ui.text}>
              The provider rotated this device token. Re-enable push to
              register it with current account proof.
            </Text>
          )}
          <Button
            label="Explain and enable push notifications"
            disabled={
              !runtime.push ||
              !runtime.session.token ||
              preferences.offline ||
              !preferences.data
            }
            onPress={() =>
              showAction(async () => {
                if (!runtime.push)
                  throw new Error('Native push provider unavailable');
                const status = await runtime.push.enableByUserAction();
                if (status === 'granted') {
                  const current = preferences.data!.data.preferences;
                  await runtime.services.updatePreferences(
                    {
                      expectedRevision: current.revision,
                      channels: {
                        inapp: current.channels.includes('inapp'),
                        email: current.channels.includes('email'),
                        push: true,
                      },
                    },
                    runtime.session.signal,
                  );
                  preferences.reload();
                }
                setNotice(
                  status === 'granted'
                    ? 'This device is registered for push notifications.'
                    : 'Notification permission was denied. Enable it in device settings to retry.',
                );
                runtime.changed();
              }, setError)
            }
          />
          {!runtime.push && (
            <Text style={ui.text}>
              Push provider is unavailable. The in-app inbox remains
              accessible.
            </Text>
          )}
          {preferences.data && (
            <Card>
              <Text style={ui.text}>
                Enabled channels:{' '}
                {preferences.data.data.preferences.channels.join(', ')}
              </Text>
              <Button
                label="Disable push channel"
                disabled={preferences.offline}
                onPress={() =>
                  showAction(
                    () =>
                      runtime.services.updatePreferences(
                        {
                          expectedRevision:
                            preferences.data!.data.preferences.revision,
                          channels: {
                            inapp: true,
                            email:
                              preferences.data!.data.preferences.channels.includes(
                                'email',
                              ),
                            push: false,
                          },
                        },
                        runtime.session.signal,
                      ),
                    setError,
                    () => {
                      setNotice('Push channel disabled.');
                      preferences.reload();
                    },
                  )
                }
              />
              <Button
                label="Reload current preferences"
                disabled={preferences.loading}
                onPress={() => {
                  setError(undefined);
                  setNotice(undefined);
                  preferences.reload();
                }}
              />
            </Card>
          )}
        </>
      }
      row={(item) => (
        <Card>
          <Text style={ui.text}>
            {item.category.replaceAll('_', ' ')}
          </Text>
          <Text style={ui.subtle}>{item.summary}</Text>
          <Text style={ui.subtle}>
            {new Date(item.createdAt).toLocaleString()} ·{' '}
            {item.read ? 'Read' : 'Unread'}
          </Text>
          <Button
            label="Mark notification read"
            disabled={!!item.read || inbox.offline}
            onPress={() =>
              showAction(
                () =>
                  runtime.services.readNotification(
                    item.notificationId,
                    runtime.session.signal,
                  ),
                setError,
                inbox.reload,
              )
            }
          />
        </Card>
      )}
    />
  );
}
