import React from 'react';

import type { TooltipProps } from 'toolkit/chakra/tooltip';
import { Tooltip as BaseTooltip } from 'toolkit/chakra/tooltip';

import { NativePopoverStyles, useNativePopover } from './NativePopover';

export const Tooltip = React.forwardRef<HTMLDivElement, TooltipProps>(function Tooltip(props, ref) {
  const [ open, setOpen ] = React.useState(props.defaultOpen ?? false);
  const { onOpenChange } = props;
  const setTooltipOpen = React.useCallback((nextOpen: boolean) => {
    setOpen(nextOpen);
    onOpenChange?.({ open: nextOpen });
  }, [ onOpenChange ]);
  const { nativePopover, content, attachContent: attachNativeContent } = useNativePopover(props.open ?? open, setTooltipOpen, 'hint');
  const hintId = React.useId();
  const generatedTriggerId = React.useId();
  const triggerId = props.ids?.trigger ?? generatedTriggerId;
  const attachContent = React.useCallback((element: HTMLDivElement | null) => {
    attachNativeContent(element);
    if (typeof ref === 'function') {
      ref(element);
    } else if (ref) {
      ref.current = element;
    }
  }, [ attachNativeContent, ref ]);
  const handleOpenChange = React.useCallback((details: { open: boolean }) => {
    setTooltipOpen(details.open);
  }, [ setTooltipOpen ]);

  React.useEffect(() => {
    const trigger = document.getElementById(triggerId);
    if (!nativePopover || !trigger || !content?.parentElement) {
      return;
    }
    const hint = content.parentElement;
    if (!hint.id) {
      hint.id = hintId;
    }
    trigger.setAttribute('interestfor', hint.id);
    return () => trigger.removeAttribute('interestfor');
  }, [ content, hintId, nativePopover, triggerId ]);

  return (
    <>
      <NativePopoverStyles/>
      <BaseTooltip
        { ...props }
        ref={ attachContent }
        open={ props.open ?? open }
        onOpenChange={ handleOpenChange }
        portalled={ nativePopover ? false : props.portalled }
        lazyMount={ nativePopover ? false : props.lazyMount }
        unmountOnExit={ nativePopover ? false : props.unmountOnExit }
        ids={{ ...props.ids, trigger: triggerId }}
      />
    </>
  );
});
