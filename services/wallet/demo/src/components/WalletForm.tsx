'use client';

import { useEffect, useRef, useState, type ComponentProps } from 'react';

export function WalletForm({
  children,
  onBlur,
  onChange,
  onFocus,
  ...props
}: ComponentProps<'form'>) {
  const [announcement, setAnnouncement] = useState('');
  const frame = useRef<number | undefined>(undefined);

  useEffect(
    () => () => {
      if (frame.current !== undefined) cancelAnimationFrame(frame.current);
    },
    []
  );

  return (
    <form
      {...props}
      onFocus={(event) => {
        onFocus?.(event);
        const control = event.target;
        if (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) {
          const dialog = control.closest('dialog');
          if (dialog) {
            const bounds = dialog.getBoundingClientRect();
            const field = control.getBoundingClientRect();
            if (field.top < bounds.top || field.bottom > bounds.bottom) {
              control.scrollIntoView({ block: 'nearest', inline: 'nearest' });
            }
          }
        }
      }}
      onBlur={(event) => {
        onBlur?.(event);
        const control = event.target;
        if (!(control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement))
          return;
        if (frame.current !== undefined) cancelAnimationFrame(frame.current);
        frame.current = requestAnimationFrame(() => {
          if (control.getAttribute('aria-invalid') !== 'true') return;
          const errorId = control.getAttribute('aria-describedby');
          const message = errorId ? document.getElementById(errorId)?.textContent : null;
          if (message) setAnnouncement(message);
        });
      }}
      onChange={(event) => {
        onChange?.(event);
        const control = event.target;
        if (
          (control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) &&
          control.validity.valid
        )
          setAnnouncement('');
      }}
    >
      {children}
      <span role="status" aria-atomic="true" className="visually-hidden">
        {announcement}
      </span>
    </form>
  );
}
