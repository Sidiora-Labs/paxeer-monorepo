import { Box, Flex, Portal, chakra } from '@chakra-ui/react';
import React from 'react';

import { CloseButton } from 'toolkit/chakra/close-button';
import { Heading } from 'toolkit/chakra/heading';

interface Props {
  title: string;
  trigger: React.ReactElement<React.AriaAttributes & { onClick?: React.MouseEventHandler }>;
  children: React.ReactNode;
}

const ActivityDialog = ({ title, trigger, children }: Props) => {
  const ref = React.useRef<HTMLDialogElement>(null);
  const titleId = React.useId();
  const dialogId = React.useId();
  const [ opened, setOpened ] = React.useState(false);

  const open = React.useCallback((event: React.MouseEvent) => {
    trigger.props.onClick?.(event);
    if (!event.defaultPrevented) {
      ref.current?.showModal();
      setOpened(true);
    }
  }, [ trigger ]);

  const close = React.useCallback(() => ref.current?.close(), []);
  const handleClose = React.useCallback(() => setOpened(false), []);

  return (
    <>
      { React.cloneElement(trigger, { onClick: open, 'aria-haspopup': 'dialog', 'aria-controls': dialogId, 'aria-expanded': opened }) }
      <Portal>
        <chakra.dialog
          ref={ ref }
          id={ dialogId }
          onClose={ handleClose }
          aria-labelledby={ titleId }
          bg="dialog.bg"
          color="dialog.fg"
          p={ 6 }
          m={ 0 }
          borderWidth={ 0 }
          w="full"
          maxW="full"
          minH="100dvh"
          maxH="100dvh"
          overflowY="auto"
          overscrollBehavior="contain"
          css={{
            '&[open]': { display: 'flex', flexDirection: 'column' },
            '& button': { minW: 'touchTarget', minH: 'touchTarget' },
          }}
        >
          <Flex gap={ 2 } alignItems="center" flexWrap="wrap" mb={ 4 }>
            <Heading
              id={ titleId } level="2" minW={ 0 } maxW="full" flexBasis="var(--chakra-spacing-48)" flexGrow={ 1 } flexShrink={ 0 } textWrap="balance"
            >
              { title }
            </Heading>
            <CloseButton type="button" onClick={ close } ml="auto"/>
          </Flex>
          <Box minW={ 0 }>{ opened && children }</Box>
        </chakra.dialog>
      </Portal>
    </>
  );
};

export default ActivityDialog;
