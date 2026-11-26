'use client';

import { Search } from 'lucide-react';
import { Button, EmptyState } from '@/ui/primitives';

export default function NotFound() {
  return (
    <div className="mx-auto w-full max-w-120 px-4 py-4">
      <EmptyState
        icon={Search}
        headingLevel={1}
        title="Page not found"
        body="We couldn't find that page"
        action={<Button variant="secondary" href="/">Go home</Button>}
      />
    </div>
  );
}
