import { chakra, DialogRootProvider, useDialog, useDialogContext, useDialogStyles } from '@chakra-ui/react';
import React from 'react';

import { DialogContent as BaseDialogContent, DialogRoot as BaseDialogRoot } from 'toolkit/chakra/dialog';

export { DialogActionTrigger, DialogBody, DialogCloseTrigger, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from 'toolkit/chakra/dialog';

const NativeDialogContext = React.createContext(false);

function subscribeToDialogSupport() {
  return () => undefined;
}

function supportsNativeDialog() {
  return typeof HTMLDialogElement !== 'undefined' && typeof HTMLDialogElement.prototype.showModal === 'function';
}

function NativeDialogRoot(props: React.ComponentProps<typeof BaseDialogRoot>) {
  const [ uncontrolledOpen, setUncontrolledOpen ] = React.useState(props.defaultOpen ?? false);
  const open = props.open ?? uncontrolledOpen;
  const { onOpenChange } = props;
  const controlled = props.open !== undefined;
  const base = useDialog({ ...props, open: false });
  const contentId = base.getContentProps().id;
  const setOpen = React.useCallback((nextOpen: boolean) => {
    const dialog = contentId ? document.getElementById(contentId) : null;
    if (!nextOpen && dialog instanceof HTMLDialogElement && dialog.open) {
      dialog.close();
    }
    if (!controlled) {
      setUncontrolledOpen(nextOpen);
    }
    onOpenChange?.({ open: nextOpen });
  }, [ contentId, controlled, onOpenChange ]);
  const value: typeof base = {
    ...base,
    open,
    setOpen,
    getTriggerProps() {
      return {
        ...base.getTriggerProps(),
        'aria-expanded': open,
        'data-state': open ? 'open' : 'closed',
        onClick(event) {
          if (!event.defaultPrevented) {
            setOpen(!open);
          }
        },
      };
    },
    getCloseTriggerProps() {
      return {
        ...base.getCloseTriggerProps(),
        onClick(event) {
          if (!event.defaultPrevented) {
            setOpen(false);
          }
        },
      };
    },
  };

  return <DialogRootProvider { ...props } value={ value }/>;
}

export function DialogRoot(props: React.ComponentProps<typeof BaseDialogRoot>) {
  const nativeDialog = React.useSyncExternalStore(subscribeToDialogSupport, supportsNativeDialog, () => false);

  return (
    <NativeDialogContext.Provider value={ nativeDialog }>
      { nativeDialog ? <NativeDialogRoot { ...props }/> : <BaseDialogRoot { ...props }/> }
    </NativeDialogContext.Provider>
  );
}

export function DialogContent(props: React.ComponentProps<typeof BaseDialogContent>) {
  const nativeDialog = React.useContext(NativeDialogContext);
  const context = useDialogContext();
  const { open, setOpen } = context;
  const styles = useDialogStyles();
  const { backdrop, portalled, portalRef, css, ...rest } = props;
  const contentProps = context.getContentProps();
  const [ dialog, setDialog ] = React.useState<HTMLDialogElement | null>(null);

  const attachDialog = React.useCallback((element: HTMLDialogElement | null) => {
    if (element) {
      element.setAttribute('closedby', 'any');
      setDialog(element);
    } else {
      setDialog(null);
    }
  }, []);

  React.useEffect(() => {
    if (!nativeDialog || !dialog) {
      return;
    }
    if (open && !dialog.open) {
      dialog.showModal();
    } else if (!open && dialog.open) {
      dialog.close();
    }
  }, [ nativeDialog, dialog, open ]);

  React.useEffect(() => {
    if (!dialog) {
      return;
    }
    const handleClose = () => {
      if (!dialog.open) {
        dialog.returnValue = '';
        setOpen(false);
      }
    };
    dialog.addEventListener('close', handleClose);
    return () => dialog.removeEventListener('close', handleClose);
  }, [ dialog, setOpen ]);

  if (!nativeDialog) {
    return <BaseDialogContent { ...props }/>;
  }

  return (
    <chakra.dialog
      { ...rest }
      ref={ attachDialog }
      role={ undefined }
      aria-modal={ undefined }
      id={ contentProps.id }
      aria-labelledby={ props['aria-labelledby'] ?? contentProps['aria-labelledby'] }
      aria-describedby={ props['aria-describedby'] ?? contentProps['aria-describedby'] }
      data-scope="dialog"
      data-part="content"
      data-state={ open ? 'open' : 'closed' }
      position="fixed"
      inset={ 0 }
      m="auto"
      w="full"
      borderWidth={ 0 }
      maxH="100dvh"
      overflowY="auto"
      overscrollBehavior="contain"
      css={ [ styles.content, css ?? {}, {
        '&:not([open])': { display: 'none' },
        '& [data-scope="dialog"][data-part="title"]': { whiteSpace: 'normal', overflow: 'visible', overflowWrap: 'anywhere', minWidth: 0 },
        '& [data-scope="dialog"][data-part="close-trigger"]': { flexShrink: 0 },
        '&::backdrop': { bg: 'blackAlpha.800', overflow: 'hidden', overscrollBehavior: 'contain' },
      } ] }
    />
  );
}
