'use client';
import ConversationList from '@/widgets/chat/conversation-list';
export default function ChatModule() {
  return (
    <div className="mx-auto flex w-full max-w-content-narrow-max-width flex-col gap-4 px-4 py-4 text-text md:px-6">
      <h1 className="text-lg font-extrabold">Messages</h1>
      <ConversationList />
    </div>
  );
}
