'use client';
import ConversationList from '@/widgets/chat/conversation-list';
export default function ChatModule() {
  return (
    <div className="mx-auto max-w-2xl p-6 text-text">
      <h1 className="mb-6 text-lg font-bold">Messages</h1>
      <ConversationList />
    </div>
  );
}
