import { useMenuContext } from '@chakra-ui/react';
import React from 'react';

import { MenuContent as BaseMenuContent } from 'toolkit/chakra/menu';

import { NativePopoverStyles, useNativePopover } from './NativePopover';

export { MenuItem, MenuRoot, MenuTrigger } from 'toolkit/chakra/menu';

export const MenuContent = React.forwardRef<HTMLDivElement, React.ComponentProps<typeof BaseMenuContent>>(function MenuContent(
  { portalled, portalRef, ...props }, ref,
) {
  const { open, setOpen, highlightedValue } = useMenuContext();
  const { nativePopover, content, attachContent: attachNativeContent } = useNativePopover(open, setOpen);
  const attachContent = React.useCallback((element: HTMLDivElement | null) => {
    attachNativeContent(element);
    if (typeof ref === 'function') {
      ref(element);
    } else if (ref) {
      ref.current = element;
    }
  }, [ attachNativeContent, ref ]);

  React.useLayoutEffect(() => {
    if (!nativePopover || !open || !content || !highlightedValue) {
      return;
    }
    const item = Array.from(content.querySelectorAll<HTMLElement>('[role="menuitem"]'))
      .find((element) => element.dataset.value === highlightedValue);
    if (item) {
      item.tabIndex = -1;
      item.focus();
    }
  }, [ content, highlightedValue, nativePopover, open ]);

  return (
    <>
      <NativePopoverStyles/>
      <BaseMenuContent
        ref={ attachContent }
        portalled={ nativePopover ? false : portalled }
        portalRef={ portalRef }
        { ...props }
      />
    </>
  );
});
