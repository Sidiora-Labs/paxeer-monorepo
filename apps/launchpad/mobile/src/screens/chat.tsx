import React, { useState } from 'react';
import { Text } from 'react-native';
import type { ChatConversation, ChatMessage } from '../runtime/routes/chat.ts';
import { nativeRequestId } from '../platform/protected-storage.ts';
import {
  Button,
  Card,
  ErrorNotice,
  Field,
  ListPage,
  ResourceStatus,
  showAction,
  useResource,
  useRetainedInput,
  ui,
  type Runtime,
} from '../navigation.tsx';
type Row =
  | { kind: 'conversation'; value: ChatConversation }
  | { kind: 'message'; value: ChatMessage };
export function Chat({ runtime }: { runtime: Runtime }) {
  const conversations = useResource(
    runtime,
    'conversations',
    (s) => runtime.services.conversations(s),
    true,
  );
  const [room, setRoom] = useRetainedInput(runtime, 'private:chat', 'room'),
    [body, setBody] = useRetainedInput(runtime, `private:chat:${room}`, 'body'),
    [error, setError] = useState<string>();
  const messages = useResource(
    runtime,
    `messages:${room}`,
    (s) =>
      room ? runtime.services.messages(room, s) : Promise.resolve(undefined),
    true,
  );
  const rows: Row[] = room
    ? (messages.data?.data.items ?? []).map((value) => ({
        kind: 'message',
        value,
      }))
    : (conversations.data?.data.items ?? []).map((value) => ({
        kind: 'conversation',
        value,
      }));
  return (
    <ListPage
      runtime={runtime}
      stateKey={`private:chat:${room}`}
      title="Messages"
      data={rows}
      keyOf={(row) =>
        row.kind === 'message' ? row.value.messageId : row.value.conversationId
      }
      header={
        <>
          <ResourceStatus {...(room ? messages : conversations)} />
          {error && <ErrorNotice message={error} />}{' '}
          {room && (
            <>
              <Button
                label="Back to conversations"
                onPress={() => setRoom('')}
              />
              <Field label="Message" value={body} onChange={setBody} />
              <Button
                label="Send message"
                disabled={!body.trim() || messages.offline}
                onPress={() =>
                  void showAction(
                    async () => {
                      await runtime.services.sendMessage(
                        room,
                        body,
                        await nativeRequestId(),
                        runtime.session.signal,
                      );
                      setBody('');
                    },
                    setError,
                    messages.reload,
                  )
                }
              />
            </>
          )}
        </>
      }
      row={(row) =>
        row.kind === 'message' ? (
          <Card>
            <Text selectable style={ui.subtle}>
              {row.value.authorWallet}
            </Text>
            <Text style={ui.text}>
              {row.value.deletedAt ? 'Message deleted' : row.value.body}
            </Text>
          </Card>
        ) : (
          <Card>
            <Text style={ui.text}>
              {row.value.peer} · {row.value.unreadCount} unread
            </Text>
            <Button
              label="Open conversation"
              onPress={(
                (conversationId: string) => () =>
                  setRoom(conversationId)
              )(row.value.conversationId)}
            />
          </Card>
        )
      }
    />
  );
}
