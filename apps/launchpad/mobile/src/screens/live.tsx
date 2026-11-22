import React, { useState } from 'react';
import { requireNativeComponent, type ViewProps } from 'react-native';
import {
  Button,
  Text,
  Card,
  ErrorNotice,
  ListPage,
  ResourceStatus,
  PageControls,
  usePagination,
  useResource,
  useRetainedInput,
  useSurface,
  type Runtime,
} from '../navigation.tsx';
import { pageResourceKey } from '../runtime/flow-state.ts';
const NativeVideo = requireNativeComponent<
  ViewProps & { source: string }
>('SidioraVideoView');
function managedURL(runtime: Runtime, value: string) {
  const url = new URL(value);
  if (
    url.protocol !== 'https:' ||
    url.username ||
    url.password ||
    !runtime.services.config.approvedPlaybackOrigins.includes(url.origin)
  )
    throw new Error('Managed HTTPS playback required');
  return url.toString();
}
export function Live({ runtime }: { runtime: Runtime }) {
  const { ui } = useSurface();
  const page = usePagination('live');
  const state = useResource(
      runtime,
      pageResourceKey('live', page.cursor),
      (s) => runtime.services.streams(s, page.cursor),
    ),
    [playback, setPlayback] = useRetainedInput(
      runtime,
      'live',
      'playback',
    ),
    [error, setError] = useState<string>();
  const items =
    state.data?.data.items.map(
      ({ streamId, title, status, playbackUrl }) => ({
        streamId,
        title,
        status,
        playbackUrl,
      }),
    ) ?? [];
  return (
    <ListPage
      runtime={runtime}
      stateKey="live"
      title="Live streams"
      data={items}
      keyOf={(item) => item.streamId}
      header={
        <>
          <ResourceStatus {...state} />
          <PageControls
            page={page}
            nextCursor={state.data?.nextCursor}
            loading={state.loading}
            offline={state.offline}
          />
          {error && <ErrorNotice message={error} />}
          {playback && (
            <Card>
              <NativeVideo
                source={playback}
                style={ui.video}
                accessibilityLabel="Managed livestream player"
              />
              <Button
                label="Close playback"
                onPress={() => setPlayback('')}
              />
            </Card>
          )}
        </>
      }
      row={(item) => (
        <Card>
          <Text style={ui.text}>{item.title}</Text>
          <Text style={ui.subtle}>{item.status}</Text>
          <Button
            label="Watch livestream"
            disabled={!item.playbackUrl || item.status !== 'live'}
            onPress={() => {
              try {
                setPlayback(managedURL(runtime, item.playbackUrl!));
              } catch (failure) {
                setError(
                  failure instanceof Error
                    ? failure.message
                    : 'Playback unavailable',
                );
              }
            }}
          />
        </Card>
      )}
    />
  );
}
