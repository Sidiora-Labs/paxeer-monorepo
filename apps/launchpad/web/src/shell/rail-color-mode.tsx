'use client';

import { useState } from 'react';
import { SunMoon } from 'lucide-react';
import { ColorModeControl, Sheet } from '@/ui/primitives';
import { cn } from '@/lib/utils';

export default function RailColorMode({ expanded, className }: { expanded: boolean; className?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        aria-label="Colour mode"
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => setOpen(true)}
        className={cn(className, expanded ? 'w-full gap-3 px-3' : 'w-10 justify-center')}
      >
        <SunMoon aria-hidden="true" className="size-icon-rail shrink-0" />
        {expanded ? <span className="truncate text-sm font-medium">Colour mode</span> : null}
      </button>
      <Sheet open={open} onClose={() => setOpen(false)} title="Colour mode" size="sm">
        <ColorModeControl />
      </Sheet>
    </>
  );
}
