import { cn } from '@/lib/utils';
import { Dialog as DialogPrimitive } from '@base-ui/react/dialog';
import { useRender } from '@base-ui/react/use-render';
import { XIcon } from 'lucide-react';
import * as React from 'react';

type DialogState = {
  open: boolean;
  setOpen: (open: boolean) => void;
  'native': boolean;
  id: string;
};
const DialogContext = React.createContext<DialogState | null>(null);

function useDialog() {
  const value = React.useContext(DialogContext);
  if (!value) throw new Error('Dialog controls require a Dialog.');
  return value;
}

function Dialog({ open, defaultOpen = false, onOpenChange, children, ...props }: Omit<DialogPrimitive.Root.Props, 'onOpenChange' | 'children'> & { children?: React.ReactNode; onOpenChange?: (open: boolean) => void }) {
  const [ inner, setInner ] = React.useState(defaultOpen);
  const [ native, setNative ] = React.useState(false);
  const id = React.useId();
  React.useEffect(() => {
    setNative(typeof HTMLDialogElement !== 'undefined' && 'showModal' in HTMLDialogElement.prototype);
  }, []);
  const setOpen = (next: boolean) => {
    if (open === undefined) setInner(next);
    onOpenChange?.(next);
  };
  const value = { open: open ?? inner, setOpen, native, id };
  return (
    <DialogContext.Provider value={ value }>
      { native ? children : <DialogPrimitive.Root open={ open } defaultOpen={ defaultOpen } onOpenChange={ onOpenChange } { ...props }>{ children }</DialogPrimitive.Root> }
    </DialogContext.Provider>
  );
}

function DialogTrigger({ render, ...props }: DialogPrimitive.Trigger.Props) {
  const state = useDialog();
  const trigger = useRender({
    defaultTagName: 'button', render,
    state: { disabled: props.disabled ?? false, open: state.open },
    props: { ...props, type: 'button', 'aria-haspopup': 'dialog', 'aria-expanded': state.open, 'aria-controls': state.id, onClick: () => state.setOpen(true) },
  });
  return state.native ? trigger : <DialogPrimitive.Trigger render={ render } { ...props }/>;
}

function DialogPortal(props: DialogPrimitive.Portal.Props) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" { ...props }/>;
}

function DialogClose({ render, ...props }: DialogPrimitive.Close.Props) {
  const state = useDialog();
  const close = useRender({ defaultTagName: 'button', render, state: { disabled: props.disabled ?? false }, props: { ...props, type: 'button', 'data-slot': 'dialog-close', onClick: () => state.setOpen(false) } });
  return state.native ? close : <DialogPrimitive.Close render={ render } data-slot="dialog-close" { ...props }/>;
}

function DialogOverlay({ className, ...props }: DialogPrimitive.Backdrop.Props) {
  return (
    <DialogPrimitive.Backdrop
      data-slot="dialog-overlay"
      className={ cn(
        'fixed inset-0 z-50 bg-overlay backdrop-blur-(--overlay-blur) transition-opacity duration-(--duration-normal) ease-(--ease-standard) data-starting-style:opacity-0 data-ending-style:opacity-0',
        className,
      ) }
      { ...props }
    />
  );
}

function LegacyDialogContent({
  className,
  children,
  showCloseButton = true,
  ...props
}: DialogPrimitive.Popup.Props & { showCloseButton?: boolean }) {
  return (
    <DialogPortal>
      <DialogOverlay/>
      <DialogPrimitive.Popup
        data-slot="dialog-content"
        className={ cn(
          'fixed top-1/2 left-1/2 z-50 grid max-h-[calc(100dvb-2*var(--overlay-gutter))] w-full max-w-[calc(100%-2*var(--overlay-gutter))] -translate-x-1/2 -translate-y-1/2 gap-5 overflow-y-auto rounded-2xl bg-popover p-6 text-popover-foreground shadow-float outline-none sm:max-w-lg',
          'transition-[opacity,scale,translate] duration-(--duration-normal) ease-(--ease-standard) data-starting-style:scale-96 data-starting-style:opacity-0 data-ending-style:scale-96 data-ending-style:opacity-0',
          'data-nested-dialog-open:scale-[calc(1-0.04*var(--nested-dialogs))]',
          className,
        ) }
        { ...props }
      >
        { children }
        { showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="state-layer focus-ring absolute top-4 right-4 inline-flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:text-foreground [&_svg]:size-4"
          >
            <XIcon/>
            <span className="sr-only">Close</span>
          </DialogPrimitive.Close>
        ) }
      </DialogPrimitive.Popup>
    </DialogPortal>
  );
}

function NativeDialogContent({ className, children, showCloseButton = true, ...props }: React.ComponentProps<'dialog'> & { showCloseButton?: boolean }) {
  const state = useDialog();
  const ref = React.useRef<HTMLDialogElement>(null);
  React.useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (state.open && !dialog.open) dialog.showModal();
    if (!state.open && dialog.open) dialog.close();
  }, [ state.open ]);
  return (
    <dialog
      { ...props }
      ref={ ref }
      id={ state.id }
      { ...{ closedby: 'any' } }
      aria-labelledby={ `${ state.id }-title` }
      data-slot="dialog-content"
      className={ cn('@container/dialog bridge-dialog gap-5 overflow-y-auto rounded-2xl bg-popover p-6 text-popover-foreground shadow-float', className) }
      onClose={ () => state.setOpen(false) }
    >
      { children }
      { showCloseButton ? (
        <form method="dialog" className="absolute top-4 right-4">
          <button type="submit" autoFocus className="state-layer focus-ring inline-flex size-8 items-center justify-center rounded-full text-muted-foreground hover:text-foreground">
            <XIcon className="size-4" aria-hidden="true"/>
            <span className="sr-only">Close</span>
          </button>
        </form>
      ) : null }
    </dialog>
  );
}

function DialogContent(props: React.ComponentProps<typeof LegacyDialogContent>) {
  const state = useDialog();
  if (!state.native) return <LegacyDialogContent { ...props }/>;
  const { className, children, showCloseButton } = props;
  return <NativeDialogContent className={ typeof className === 'string' ? className : undefined } showCloseButton={ showCloseButton }>{ children }</NativeDialogContent>;
}

function DialogHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="dialog-header" className={ cn('flex flex-col gap-1.5 text-left', className) } { ...props }/>;
}

function DialogFooter({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="dialog-footer"
      className={ cn('flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className) }
      { ...props }
    />
  );
}

function DialogTitle({ className, ...props }: DialogPrimitive.Title.Props) {
  const state = useDialog();
  if (state.native) return <h2 id={ `${ state.id }-title` } className={ cn('min-h-8 pr-10 text-title font-medium', typeof className === 'string' ? className : undefined) }>{ props.children }</h2>;
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={ cn('min-h-8 pr-10 text-title leading-tight font-medium tracking-(--tracking-title)', className) }
      { ...props }
    />
  );
}

function DialogDescription({ className, ...props }: DialogPrimitive.Description.Props) {
  const state = useDialog();
  if (state.native) return <p className={ cn('text-muted-foreground text-sm leading-relaxed', typeof className === 'string' ? className : undefined) }>{ props.children }</p>;
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={ cn('text-muted-foreground text-sm leading-relaxed', className) }
      { ...props }
    />
  );
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
