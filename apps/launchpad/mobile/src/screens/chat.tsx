import React, { useState } from 'react';
import type { ChatConversation } from '../runtime/routes/chat.ts';
import type { DurableMessage } from '../runtime/chat-session.ts';
import { pageResourceKey } from '../runtime/flow-state.ts';
import { nativeRequestId } from '../platform/protected-storage.ts';
import {
  Button,
  Text,
  Card,
  ErrorNotice,
  Field,
  ListPage,
  ResourceStatus,
  PageControls,
  usePagination,
  showAction,
  useResource,
  useRetainedInput,
  useSurface,
  type Runtime,
} from '../navigation.tsx';
type Row =
  | { kind: 'conversation'; value: ChatConversation }
  | { kind: 'message'; value: DurableMessage };
export function Chat({ runtime }: { runtime: Runtime }) {
  const { ui } = useSurface();
  const conversations = useResource(
    runtime,
    'conversations',
    (s) => runtime.services.conversations(s),
    true,
  );
  const [room, setRoom] = useRetainedInput(
      runtime,
      'private:chat',
      'room',
    ),
    [body, setBody] = useRetainedInput(
      runtime,
      `private:chat:${room}`,
      'body',
    ),
    [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [messageId, setMessageId] = useRetainedInput(
    runtime,
    `private:chat:${room}`,
    'clientMessageId',
  );
  const [pendingBody, setPendingBody] = useRetainedInput(
    runtime,
    `private:chat:${room}`,
    'pendingBody',
  );
  const page = usePagination(`chat:${room}`);
  const messages = useResource(
    runtime,
    pageResourceKey(`messages:${room}`, page.cursor),
    (s) =>
      room
        ? runtime.services.messages(room, s, page.cursor)
        : Promise.resolve(undefined),
    true,
  );
  const rows: Row[] = room
    ? (messages.data?.messages ?? []).map((value) => ({
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
        row.kind === 'message' ? row.value.id : row.value.conversationId
      }
      header={
        <>
          <ResourceStatus {...(room ? messages : conversations)} />
          {error && <ErrorNotice message={error} />}
          {notice && <Text style={ui.text}>{notice}</Text>}
          {room && (
            <>
              <Button
                label="Back to conversations"
                onPress={() => {
                  setRoom('');
                  setError(undefined);
                  setNotice(undefined);
                }}
              />
              <PageControls
                page={page}
                nextCursor={
                  messages.data?.messages.length === 50
                    ? messages.data.next_cursor
                    : null
                }
                loading={messages.loading}
                offline={messages.offline}
              />
              <Button
                label="Refresh conversation"
                disabled={messages.loading}
                onPress={messages.reload}
              />
              <Field
                multiline
                label="Message"
                value={body}
                onChange={setBody}
              />
              <Button
                label="Send message"
                disabled={!body.trim() || messages.offline}
                onPress={() =>
                  showAction(
                    async () => {
                      if (messageId && pendingBody !== body)
                        throw new Error(
                          'Restore the pending message before retrying its delivery.',
                        );
                      const id = messageId || (await nativeRequestId());
                      setMessageId(id);
                      setPendingBody(body);
                      const result = await runtime.services.sendMessage(
                        room,
                        body,
                        id,
                        runtime.session.signal,
                      );
                      setBody('');
                      setMessageId('');
                      setPendingBody('');
                      setNotice(
                        `Message saved at conversation sequence ${result.sequence}.`,
                      );
                    },
                    setError,
                    messages.reload,
                  )
                }
              />
              {messageId && (
                <Button
                  label="Restore pending message"
                  onPress={() => setBody(pendingBody)}
                />
              )}
            </>
          )}
        </>
      }
      row={(row) =>
        row.kind === 'message' ? (
          <Card>
            <Text selectable style={ui.subtle}>
              {row.value.sender}
            </Text>
            <Text style={ui.text}>
              {row.value.deleted ? 'Message deleted' : row.value.content}
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
