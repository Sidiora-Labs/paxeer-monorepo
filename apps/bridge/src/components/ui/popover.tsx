import { cn } from '@/lib/utils';
import { Popover as PopoverPrimitive } from '@base-ui/react/popover';
import { useRender } from '@base-ui/react/use-render';
import * as React from 'react';

type PopoverState = {
  'native': boolean;
  open: boolean;
  setOpen: (open: boolean) => void;
  id: string;
  trigger: React.RefObject<HTMLElement | null>;
};
const PopoverContext = React.createContext<PopoverState | null>(null);
function usePopover() {
  const value = React.useContext(PopoverContext);
  if (!value) throw new Error('Popover controls require a Popover.');
  return value;
}

function Popover({ open, defaultOpen = false, onOpenChange, children, ...props }: Omit<PopoverPrimitive.Root.Props, 'onOpenChange' | 'children'> & { children?: React.ReactNode; onOpenChange?: (open: boolean) => void }) {
  const [ inner, setInner ] = React.useState(defaultOpen);
  const [ native, setNative ] = React.useState(false);
  const trigger = React.useRef<HTMLElement>(null);
  const id = React.useId();
  React.useEffect(() => {
    setNative('showPopover' in HTMLElement.prototype);
  }, []);
  const setOpen = (next: boolean) => {
    if (open === undefined) setInner(next);
    onOpenChange?.(next);
  };
  return (
    <PopoverContext.Provider value={{ native, open: open ?? inner, setOpen, id, trigger }}>
      { native ? children : <PopoverPrimitive.Root open={ open } defaultOpen={ defaultOpen } onOpenChange={ onOpenChange } { ...props }>{ children }</PopoverPrimitive.Root> }
    </PopoverContext.Provider>
  );
}

function PopoverTrigger({ render, ...props }: PopoverPrimitive.Trigger.Props) {
  const state = usePopover();
  const trigger = useRender({
    defaultTagName: 'button', render, ref: state.trigger,
    state: { disabled: props.disabled ?? false, open: state.open },
    props: { ...props, type: 'button', popoverTarget: state.id, 'data-slot': 'popover-trigger', 'aria-expanded': state.open },
  });
  return state.native ? trigger : <PopoverPrimitive.Trigger render={ render } { ...props }/>;
}

function LegacyPopoverContent({
  className,
  align = 'center',
  side = 'bottom',
  sideOffset = 8,
  ...props
}: PopoverPrimitive.Popup.Props &
  Pick<PopoverPrimitive.Positioner.Props, 'align' | 'side' | 'alignOffset'> & { sideOffset?: number }) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner className="z-50" align={ align } side={ side } sideOffset={ sideOffset }>
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={ cn(
            'popup-motion max-h-[calc(100dvb-2*var(--overlay-gutter))] w-72 max-w-[calc(100vw-2*var(--overlay-gutter))] overflow-y-auto rounded-lg border border-outline-variant bg-popover p-4 text-popover-foreground shadow-float outline-none',
            className,
          ) }
          { ...props }
        />
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  );
}

function NativePopoverContent({ className, children, align = 'center', side = 'bottom', sideOffset, ...props }: React.ComponentProps<'div'> & { align?: 'start' | 'center' | 'end'; side?: string; sideOffset?: number }) {
  const state = usePopover();
  const ref = React.useRef<HTMLDivElement>(null);
  React.useLayoutEffect(() => {
    const popup = ref.current;
    if (!popup) return;
    if (state.open && !popup.matches(':popover-open')) popup.showPopover();
    if (!state.open && popup.matches(':popover-open')) popup.hidePopover();
  }, [ state.open, state.trigger, align, side, sideOffset ]);
  return (
    <div
      { ...props }
      id={ state.id }
      ref={ ref }
      popover="auto"
      data-slot="popover-content"
      data-align={ align }
      data-side={ side }
      style={ sideOffset === undefined ? undefined : { margin: sideOffset } }
      className={ cn('bridge-popover w-72 overflow-y-auto rounded-lg border border-outline-variant bg-popover p-4 text-popover-foreground shadow-float', className) }
      onToggle={ (event) => state.setOpen(event.newState === 'open') }
    >
      { children }
    </div>
  );
}

function PopoverContent(props: React.ComponentProps<typeof LegacyPopoverContent>) {
  const state = usePopover();
  if (!state.native) return <LegacyPopoverContent { ...props }/>;
  const { className, children, align, side, sideOffset } = props;
  return <NativePopoverContent className={ typeof className === 'string' ? className : undefined } align={ align } side={ side } sideOffset={ sideOffset }>{ children }</NativePopoverContent>;
}

function PopoverHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="popover-header" className={ cn('mb-3 flex flex-col gap-1', className) } { ...props }/>;
}

function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props) {
  const state = usePopover();
  if (state.native) return <h2 className={ cn('text-body font-medium', typeof className === 'string' ? className : undefined) }>{ props.children }</h2>;
  return <PopoverPrimitive.Title data-slot="popover-title" className={ cn('text-body font-medium', className) } { ...props }/>;
}

function PopoverDescription({ className, ...props }: PopoverPrimitive.Description.Props) {
  const state = usePopover();
  if (state.native) return <p className={ cn('text-body-sm text-muted-foreground', typeof className === 'string' ? className : undefined) }>{ props.children }</p>;
  return <PopoverPrimitive.Description data-slot="popover-description" className={ cn('text-body-sm text-muted-foreground', className) } { ...props }/>;
}

function PopoverClose(props: React.ComponentProps<'button'>) {
  const state = usePopover();
  return state.native ? <button type="button" popoverTarget={ state.id } popoverTargetAction="hide" { ...props }/> : <PopoverPrimitive.Close { ...props }/>;
}

export { Popover, PopoverTrigger, PopoverContent, PopoverHeader, PopoverTitle, PopoverDescription, PopoverClose };
