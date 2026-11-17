import type { HTMLChakraProps } from '@chakra-ui/react';
import { Box } from '@chakra-ui/react';
import React from 'react';

interface Props extends Omit<HTMLChakraProps<'div'>, 'onToggle'> {
  open: boolean;
  onOpenChange: (details: { open: boolean }) => void;
  anchorRef: React.RefObject<HTMLElement | null>;
  anchorName: string;
  matchAnchorWidth?: boolean;
}

const supportsNativePopover = () => typeof HTMLElement !== 'undefined' &&
  'showPopover' in HTMLElement.prototype &&
  typeof CSS !== 'undefined' &&
  typeof CSS.supports === 'function' &&
  CSS.supports('selector(:popover-open)');

const NativePopover = ({ open, onOpenChange, anchorRef, anchorName, matchAnchorWidth, children, ...rest }: Props) => {
  const contentRef = React.useRef<HTMLDivElement>(null);
  const nativePopover = supportsNativePopover();
  const handleToggle = React.useCallback((event: React.SyntheticEvent<HTMLDivElement>) => {
    if ((event.nativeEvent as ToggleEvent).newState === 'closed') onOpenChange({ open: false });
  }, [ onOpenChange ]);

  React.useEffect(() => {
    const content = contentRef.current;
    if (!content) return;

    const dismissOutside = (event: Event) => {
      if (event.type === 'focusin' && event.target === document.body && anchorRef.current?.querySelector('input')) return;
      if (!content.contains(event.target as Node) && !anchorRef.current?.contains(event.target as Node)) {
        if (nativePopover && content.matches(':popover-open')) content.hidePopover();
        onOpenChange({ open: false });
      }
    };

    if (nativePopover) {
      const isShown = content.matches(':popover-open');
      if (open && !isShown) {
        const source = anchorRef.current?.querySelector<HTMLElement>('input, button') ?? anchorRef.current;
        Reflect.apply(content.showPopover, content, source ? [ { source } ] : []);
      }
      if (!open && isShown) content.hidePopover();
      if (!open) return;
      document.addEventListener('focusin', dismissOutside, true);
      return () => document.removeEventListener('focusin', dismissOutside, true);
    }

    if (!open) return;

    const dismissWithEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return;
      event.preventDefault();
      onOpenChange({ open: false });
      const anchor = anchorRef.current;
      (anchor?.querySelector<HTMLElement>('input, button') ?? anchor)?.focus();
    };
    document.addEventListener('pointerdown', dismissOutside);
    document.addEventListener('focusin', dismissOutside);
    document.addEventListener('keydown', dismissWithEscape);
    return () => {
      document.removeEventListener('pointerdown', dismissOutside);
      document.removeEventListener('focusin', dismissOutside);
      document.removeEventListener('keydown', dismissWithEscape);
    };
  }, [ nativePopover, open, onOpenChange, anchorRef ]);

  return (
    <Box
      ref={ contentRef }
      data-shell-popover
      popover={ nativePopover ? 'auto' : undefined }
      role="dialog"
      hidden={ !open }
      onToggle={ handleToggle }
      position="fixed"
      inset={ 0 }
      m="auto"
      bg="raised"
      color="text.primary"
      borderWidth="var(--chakra-border-widths-1)"
      borderColor="hairline"
      borderRadius="card"
      boxShadow="none"
      maxW="calc(100dvw - var(--chakra-spacing-8))"
      maxH="calc(100dvh - var(--chakra-spacing-8))"
      overflowY="auto"
      zIndex="popover"
      css={{
        ...(matchAnchorWidth && {
          '@supports (width: anchor-size(width))': { width: 'anchor-size(width)' },
        }),
        '@supports (position-area: bottom)': {
          positionAnchor: anchorName,
          positionArea: 'bottom',
          positionTryFallbacks: 'flip-inline, flip-block',
          inset: 'auto',
          margin: 0,
          marginBlockStart: 'var(--chakra-spacing-2)',
        },
      }}
      { ...rest }
    >
      { children }
    </Box>
  );
};

export default NativePopover;
