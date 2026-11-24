import { usePopoverContext } from '@chakra-ui/react';
import { Global } from '@emotion/react';
import React from 'react';

import type { PopoverContentProps } from 'toolkit/chakra/popover';
import { PopoverContent as BasePopoverContent } from 'toolkit/chakra/popover';

export type { PopoverContentProps } from 'toolkit/chakra/popover';
export { PopoverBody, PopoverCloseTriggerWrapper, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';

export function useNativePopover(open: boolean, setOpen: (open: boolean) => void, mode: 'auto' | 'hint' = 'auto') {
  const [ nativePopover, setNativePopover ] = React.useState(false);
  const [ content, setContent ] = React.useState<HTMLDivElement | null>(null);
  const positioner = content?.parentElement;

  const attachContent = React.useCallback((element: HTMLDivElement | null) => {
    setContent(element);
  }, []);

  React.useEffect(() => {
    setNativePopover('popover' in HTMLElement.prototype);
  }, []);

  React.useEffect(() => {
    if (!nativePopover || !content || !positioner) {
      return;
    }
    positioner.setAttribute('popover', mode);
    if (open && !positioner.matches(':popover-open')) {
      const triggerId = content.id.replace(/:content$/, ':trigger');
      const source = document.getElementById(triggerId);
      Reflect.apply(positioner.showPopover, positioner, source ? [ { source } ] : []);
    } else if (!open && positioner.matches(':popover-open')) {
      positioner.hidePopover();
    }
  }, [ content, mode, nativePopover, open, positioner ]);

  React.useEffect(() => {
    if (!nativePopover || !positioner) {
      return;
    }
    const handleToggle = (event: ToggleEvent) => {
      if ((event.newState === 'open') !== open) {
        setOpen(event.newState === 'open');
      }
    };
    positioner.addEventListener('toggle', handleToggle);
    return () => positioner.removeEventListener('toggle', handleToggle);
  }, [ nativePopover, open, positioner, setOpen ]);

  React.useEffect(() => {
    if (!nativePopover || !open || !content || !positioner) return;
    const source = document.getElementById(content.id.replace(/:content$/, ':trigger'));
    const dismissOutsideFocus = (event: FocusEvent) => {
      if (!(event.target instanceof Node) || positioner.contains(event.target) || source?.contains(event.target)) return;
      if (positioner.matches(':popover-open')) positioner.hidePopover();
      setOpen(false);
    };
    document.addEventListener('focusin', dismissOutsideFocus, true);
    return () => document.removeEventListener('focusin', dismissOutsideFocus, true);
  }, [ content, nativePopover, open, positioner, setOpen ]);

  return { nativePopover, content, attachContent };
}

export function NativePopoverStyles() {
  return (
    <Global styles={{
      '[data-part="positioner"][popover]': {
        margin: 0,
        padding: 0,
        borderWidth: 0,
        background: 'transparent',
        overflow: 'visible',
        '&:not(:popover-open)': { display: 'none' },
      },
    }}/>
  );
}

export const PopoverContent = React.forwardRef<HTMLDivElement, PopoverContentProps>(function PopoverContent(
  { portalled, portalRef, ...props }, ref,
) {
  const { open, setOpen } = usePopoverContext();
  const { nativePopover, attachContent: attachNativeContent } = useNativePopover(open, setOpen);
  const attachContent = React.useCallback((element: HTMLDivElement | null) => {
    attachNativeContent(element);
    if (typeof ref === 'function') {
      ref(element);
    } else if (ref) {
      ref.current = element;
    }
  }, [ attachNativeContent, ref ]);

  return (
    <>
      <NativePopoverStyles/>
      <BasePopoverContent
        ref={ attachContent }
        portalled={ nativePopover ? false : portalled }
        portalRef={ portalRef }
        { ...props }
      />
    </>
  );
});
