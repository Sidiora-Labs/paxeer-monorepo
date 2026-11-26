'use client';

import { cn } from '@/lib/utils';
import { ModalFrame, type ModalFrameProps } from './overlays';

export function Sheet({ className, children, ...props }: ModalFrameProps) {
  return (
    <ModalFrame {...props} className={cn('sl-sheet sl-floating', className)}>
      <span aria-hidden="true" className="mx-auto mb-4 block h-1 w-8 rounded-pill bg-text-faint md:hidden" />
      {children}
    </ModalFrame>
  );
}
