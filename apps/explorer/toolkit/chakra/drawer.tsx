import { Drawer as ChakraDrawer, Portal, useDrawerContext } from '@chakra-ui/react';
import * as React from 'react';

import useModalInert from '../hooks/useModalInert';
import { CloseButton } from './close-button';

interface DrawerContentProps extends ChakraDrawer.ContentProps {
  portalled?: boolean;
  portalRef?: React.RefObject<HTMLElement>;
  offset?: ChakraDrawer.ContentProps['padding'];
  backdrop?: boolean;
}

export const DrawerContent = React.forwardRef<
  HTMLDivElement,
  DrawerContentProps
>(function DrawerContent(props, ref) {
  const { children, portalled = true, portalRef, offset, backdrop = true, ...rest } = props;
  const { open } = useDrawerContext();
  const [ content, setContent ] = React.useState<HTMLDivElement | null>(null);
  useModalInert(content, open && content?.getAttribute('aria-modal') === 'true');
  const attachContent = React.useCallback((element: HTMLDivElement | null) => {
    setContent(element);
    if (typeof ref === 'function') ref(element);
    else if (ref) ref.current = element;
  }, [ ref ]);
  return (
    <Portal disabled={ !portalled } container={ portalRef }>
      { backdrop && <ChakraDrawer.Backdrop/> }
      <ChakraDrawer.Positioner padding={ offset }>
        <ChakraDrawer.Content ref={ attachContent } { ...rest } asChild={ false }>
          { children }
        </ChakraDrawer.Content>
      </ChakraDrawer.Positioner>
    </Portal>
  );
});

export const DrawerCloseTrigger = React.forwardRef<
  HTMLButtonElement,
  ChakraDrawer.CloseTriggerProps
>(function DrawerCloseTrigger(props, ref) {
  return (
    <ChakraDrawer.CloseTrigger
      position="absolute"
      top="7"
      insetEnd="5"
      { ...props }
      asChild
    >
      <CloseButton ref={ ref }/>
    </ChakraDrawer.CloseTrigger>
  );
});

export const DrawerRoot = (props: ChakraDrawer.RootProps) => {
  const { initialFocusEl, modal = true, onFocusOutside, lazyMount = true, unmountOnExit = true, ...rest } = props;
  const handleFocusOutside = React.useCallback<NonNullable<ChakraDrawer.RootProps['onFocusOutside']>>((event) => {
    onFocusOutside?.(event);
    if (modal) event.preventDefault();
  }, [ modal, onFocusOutside ]);
  return (
    <ChakraDrawer.Root
      { ...rest }
      modal={ modal }
      initialFocusEl={ initialFocusEl }
      lazyMount={ lazyMount }
      unmountOnExit={ unmountOnExit }
      onFocusOutside={ handleFocusOutside }
    />
  );
};

export const DrawerTrigger = (props: ChakraDrawer.TriggerProps) => {
  const { asChild = true, ...rest } = props;
  return <ChakraDrawer.Trigger asChild={ asChild } { ...rest }/>;
};

export const DrawerFooter = ChakraDrawer.Footer;
export const DrawerHeader = ChakraDrawer.Header;
export const DrawerBody = ChakraDrawer.Body;
export const DrawerDescription = ChakraDrawer.Description;
export const DrawerTitle = ChakraDrawer.Title;
export const DrawerActionTrigger = ChakraDrawer.ActionTrigger;
