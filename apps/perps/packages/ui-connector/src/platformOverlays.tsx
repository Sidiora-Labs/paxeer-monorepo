import React, {
  createContext,
  forwardRef,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type ComponentProps,
  type ComponentType,
  type PropsWithChildren,
} from "react";
import {
  cn,
  modal,
  SimpleDialogFooter,
  useModal,
  type SimpleDialogProps,
  type PopoverRoot as SurfacePopoverRoot,
  type PopoverTrigger as SurfacePopoverTrigger,
  type PopoverContent as SurfacePopoverContent,
} from "@paxeer-network/ui";

type SimpleSheetProps = ComponentProps<
  typeof import("@paxeer-network/ui").SimpleSheet
>;

/** Workspace overlays use the browser's top layer and dismissal behavior. */
export function SimpleDialog(props: PropsWithChildren<SimpleDialogProps>) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const descriptionId = useId();
  const title = typeof props.title === "function" ? props.title() : props.title;
  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (props.open && !dialog.open) dialog.showModal();
    else if (!props.open && dialog.open) dialog.close();
  }, [props.open]);

  return (
    <dialog
      ref={ref}
      {...{ closedby: props.onOpenChange ? "any" : "none" }}
      aria-labelledby={title ? titleId : undefined}
      aria-label={
        title ? undefined : (props.contentProps?.["aria-label"] ?? "Dialog")
      }
      aria-describedby={props.description ? descriptionId : undefined}
      className={cn(
        "oui-platform-dialog oui-shell-overlay",
        props.classNames?.content,
        props.contentProps?.className,
      )}
      data-size={props.size ?? "md"}
      style={props.contentProps?.style}
      onCancel={(event) => {
        if (!props.onOpenChange) event.preventDefault();
      }}
      onClose={() => {
        if (!ref.current?.open) {
          if (ref.current) ref.current.returnValue = "";
          props.onOpenChange?.(false);
        }
      }}
    >
      <header className="oui-flex oui-items-center oui-justify-between oui-gap-3 oui-border-b oui-border-line-12 oui-pb-3">
        <h2
          id={titleId}
          className={cn(
            "oui-min-w-0 oui-flex-1 oui-text-base oui-font-semibold",
            !title && "oui-sr-only",
          )}
        >
          {title ?? "Dialog"}
        </h2>
        {props.closable !== false && (
          <button
            type="button"
            className="oui-shell-icon-button oui-ms-auto"
            aria-label="Close"
            onClick={() => ref.current?.close()}
          >
            <svg
              aria-hidden="true"
              width="24"
              height="24"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path d="m6 6 12 12M18 6 6 18" />
            </svg>
          </button>
        )}
      </header>
      <div className={cn("oui-py-4", props.classNames?.body)}>
        {props.open ? props.children : null}
      </div>
      {props.description && (
        <p id={descriptionId} className="oui-text-xs oui-text-base-contrast-80">
          {props.description}
        </p>
      )}
      {props.open && (
        <SimpleDialogFooter
          actions={props.actions}
          className={props.classNames?.footer}
        />
      )}
    </dialog>
  );
}

export function SimpleSheet(props: PropsWithChildren<SimpleSheetProps>) {
  return (
    <SimpleDialog
      {...props}
      closable={props.contentProps?.closeable ?? props.closable}
      title={props.title}
      classNames={{
        ...props.classNames,
        content: cn("oui-platform-sheet", props.classNames?.content),
      }}
    />
  );
}

export function registerSimpleDialog<Props extends object = {}>(
  id: string,
  Component: ComponentType<Props>,
  props?: Partial<SimpleDialogProps & Props>,
) {
  const Overlay = modal.create((overrides: SimpleDialogProps & Props) => {
    const options = { ...props, ...overrides };
    const { visible, hide, resolve, reject, onOpenChange } = useModal();
    return (
      <SimpleDialog {...options} open={visible} onOpenChange={onOpenChange}>
        <Component
          {...options}
          close={hide}
          resolve={resolve}
          reject={reject}
        />
      </SimpleDialog>
    );
  });
  modal.register(id, Overlay);
}

export function registerSimpleSheet<Props extends object = {}>(
  id: string,
  Component: ComponentType<Props>,
  props?: Partial<SimpleSheetProps & Props>,
) {
  const Overlay = modal.create((overrides: SimpleSheetProps & Props) => {
    const options = { ...props, ...overrides };
    const { visible, hide, resolve, reject, onOpenChange } = useModal();
    return (
      <SimpleSheet {...options} open={visible} onOpenChange={onOpenChange}>
        <Component
          {...options}
          close={hide}
          resolve={resolve}
          reject={reject}
        />
      </SimpleSheet>
    );
  });
  modal.register(id, Overlay);
}

type PopoverState = {
  id: string;
  open: boolean;
  change: (open: boolean) => void;
};
const PopoverContext = createContext<PopoverState | null>(null);
function usePopover() {
  const context = useContext(PopoverContext);
  if (!context) throw new Error("Popover content requires a popover root.");
  return context;
}

export function PopoverRoot(props: ComponentProps<typeof SurfacePopoverRoot>) {
  const id = `oui-popover-${useId().replace(/:/g, "")}`;
  const [localOpen, setLocalOpen] = useState(props.defaultOpen ?? false);
  const open = props.open ?? localOpen;
  return (
    <PopoverContext.Provider
      value={{
        id,
        open,
        change(next) {
          setLocalOpen(next);
          if (next !== open) props.onOpenChange?.(next);
        },
      }}
    >
      {props.children}
    </PopoverContext.Provider>
  );
}

export const PopoverTrigger = forwardRef<
  HTMLButtonElement,
  ComponentProps<typeof SurfacePopoverTrigger>
>((props, forwardedRef) => {
  const { id, open } = usePopover();
  const { asChild, children, ...rest } = props;
  const menu =
    props["aria-haspopup"] === "menu" ||
    (React.isValidElement<ComponentProps<"button">>(children) &&
      children.props["aria-haspopup"] === "menu");
  const attributes = {
    ...rest,
    ref: forwardedRef,
    ...(menu
      ? { commandfor: id, command: "toggle-popover" }
      : { popovertarget: id }),
    "aria-expanded": open,
    "data-state": open ? "open" : "closed",
    className: cn("oui-platform-popover-trigger", props.className),
    style: { ...props.style, ...{ "--oui-popover-anchor": `--${id}` } },
    onClick(event: React.MouseEvent<HTMLButtonElement>) {
      if (asChild && React.isValidElement<ComponentProps<"button">>(children))
        children.props.onClick?.(event);
      props.onClick?.(event);
      if (!menu || event.defaultPrevented) return;
      event.currentTarget.dataset.menuKeyboard = String(event.detail === 0);
      if (!("commandForElement" in HTMLButtonElement.prototype)) {
        event.preventDefault();
        document.getElementById(id)?.togglePopover();
      }
    },
    onKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
      if (asChild && React.isValidElement<ComponentProps<"button">>(children))
        children.props.onKeyDown?.(event);
      props.onKeyDown?.(event);
      if (!menu || event.defaultPrevented || event.key !== "ArrowDown") return;
      event.preventDefault();
      event.currentTarget.dataset.menuKeyboard = "true";
      if (!open) event.currentTarget.click();
      else
        document
          .getElementById(id)
          ?.querySelector<HTMLButtonElement>("button:not(:disabled)")
          ?.focus();
    },
  };
  if (asChild && !React.isValidElement(children))
    throw new Error("Popover trigger requires one element.");
  if (asChild && React.isValidElement<ComponentProps<"button">>(children))
    return React.cloneElement(children, {
      ...attributes,
      className: cn(children.props.className, attributes.className),
      style: { ...children.props.style, ...attributes.style },
    });
  return (
    <button type="button" {...attributes}>
      {children}
    </button>
  );
});
PopoverTrigger.displayName = "PopoverTrigger";

export const PopoverContent = forwardRef<
  HTMLDivElement,
  ComponentProps<typeof SurfacePopoverContent>
>((props, forwardedRef) => {
  const { id, open, change } = usePopover();
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    const panel = ref.current;
    if (!panel) return;
    if (open && !panel.matches(":popover-open")) panel.showPopover();
    else if (!open && panel.matches(":popover-open")) panel.hidePopover();
  }, [open]);
  useEffect(() => {
    const panel = ref.current;
    if (!panel) return;
    const update = () => change(panel.matches(":popover-open"));
    panel.addEventListener("toggle", update);
    return () => panel.removeEventListener("toggle", update);
  });
  return (
    <div
      ref={(element) => {
        ref.current = element;
        if (typeof forwardedRef === "function") forwardedRef(element);
        else if (forwardedRef) forwardedRef.current = element;
      }}
      id={id}
      {...{ popover: "auto" }}
      tabIndex={-1}
      aria-label={props["aria-label"]}
      aria-labelledby={props["aria-labelledby"]}
      className={cn("oui-platform-popover oui-shell-overlay", props.className)}
      style={{ ...props.style, ...{ "--oui-popover-anchor": `--${id}` } }}
      data-side={props.side ?? "bottom"}
      data-align={props.align ?? "center"}
      data-state={open ? "open" : "closed"}
      onClick={props.onClick}
    >
      {open ? props.children : null}
    </div>
  );
});
PopoverContent.displayName = "PopoverContent";

export function Popover(
  props: PropsWithChildren<
    ComponentProps<typeof SurfacePopoverRoot> & {
      content: React.ReactNode;
      arrow?: boolean;
      contentProps?: ComponentProps<typeof SurfacePopoverContent>;
    }
  >,
) {
  return (
    <PopoverRoot {...props}>
      <PopoverTrigger asChild>{props.children}</PopoverTrigger>
      <PopoverContent {...props.contentProps}>{props.content}</PopoverContent>
    </PopoverRoot>
  );
}

export const Tooltip = forwardRef<
  HTMLDivElement,
  ComponentProps<typeof import("@paxeer-network/ui").Tooltip>
>((props, forwardedRef) => {
  const id = `oui-tooltip-${useId().replace(/:/g, "")}`;
  const [localOpen, setLocalOpen] = useState(props.defaultOpen ?? false);
  const open = props.open ?? localOpen;
  const panel = useRef<HTMLDivElement | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>();
  const change = (next: boolean) => {
    clearTimeout(timer.current);
    setLocalOpen(next);
    if (next !== open) props.onOpenChange?.(next);
  };
  const duration = (step: "fast" | "normal") => {
    const value = getComputedStyle(document.documentElement)
      .getPropertyValue(`--oui-duration-${step}`)
      .trim();
    return parseFloat(value) * (value.endsWith("ms") ? 1 : 1000);
  };
  const enter = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(
      () => change(true),
      props.delayDuration ?? duration("normal"),
    );
  };
  const leave = () => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => change(false), duration("fast"));
  };
  useEffect(() => () => clearTimeout(timer.current), []);
  useEffect(() => {
    const element = panel.current;
    if (!element) return;
    if (open && !element.matches(":popover-open")) element.showPopover();
    else if (!open && element.matches(":popover-open")) element.hidePopover();
  }, [open]);
  useEffect(() => {
    const element = panel.current;
    if (!element) return;
    const update = () => change(element.matches(":popover-open"));
    element.addEventListener("toggle", update);
    return () => element.removeEventListener("toggle", update);
  });
  const child = React.isValidElement<React.HTMLAttributes<HTMLElement>>(
    props.children,
  )
    ? props.children
    : null;
  if (!child) throw new Error("Tooltip trigger requires one element.");
  return (
    <>
      {React.cloneElement(child, {
        "aria-describedby": [child.props["aria-describedby"], id]
          .filter(Boolean)
          .join(" "),
        className: cn("oui-platform-popover-trigger", child.props.className),
        style: {
          ...child.props.style,
          ...{ "--oui-tooltip-anchor": `--${id}` },
        },
        onPointerEnter(event: React.PointerEvent<HTMLElement>) {
          child.props.onPointerEnter?.(event);
          enter();
        },
        onPointerLeave(event: React.PointerEvent<HTMLElement>) {
          child.props.onPointerLeave?.(event);
          leave();
        },
        onFocus(event: React.FocusEvent<HTMLElement>) {
          child.props.onFocus?.(event);
          enter();
        },
        onBlur(event: React.FocusEvent<HTMLElement>) {
          child.props.onBlur?.(event);
          if (!event.currentTarget.contains(event.relatedTarget)) leave();
        },
      })}
      <div
        ref={(element) => {
          panel.current = element;
          if (typeof forwardedRef === "function") forwardedRef(element);
          else if (forwardedRef) forwardedRef.current = element;
        }}
        id={id}
        role="tooltip"
        {...{ popover: "auto" }}
        className={cn(
          "oui-platform-popover oui-platform-tooltip",
          props.className,
        )}
        style={{ ...props.style, ...{ "--oui-popover-anchor": `--${id}` } }}
        data-side={props.side ?? "top"}
        data-align={props.align ?? "center"}
        onPointerEnter={() => clearTimeout(timer.current)}
        onPointerLeave={leave}
      >
        {props.content}
      </div>
    </>
  );
});
Tooltip.displayName = "Tooltip";

/** Native command menu with keyboard support for browsers without focusgroup. */
export function Menu(props: PropsWithChildren<{ label: string }>) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const group = ref.current;
    if (!group) return;
    const panel = group.closest<HTMLElement>("[popover]");
    if (!panel) return;
    const trigger = document.querySelector<HTMLButtonElement>(
      `[commandfor="${panel.id}"]`,
    );
    if (!trigger) return;
    const nativeFocusgroup = "focusGroup" in HTMLElement.prototype;
    if (!nativeFocusgroup) group.setAttribute("role", "menu");
    const items = Array.from(
      group.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"),
    );
    items.forEach((item, index) => {
      if (!nativeFocusgroup) {
        item.setAttribute("role", "menuitem");
        item.tabIndex = index === 0 ? 0 : -1;
      }
    });
    const focus = (item: HTMLButtonElement) => {
      if (!nativeFocusgroup)
        items.forEach((candidate) => {
          candidate.tabIndex = candidate === item ? 0 : -1;
        });
      item.focus();
    };
    if (trigger.dataset.menuKeyboard === "true" && items[0]) focus(items[0]);
    const close = () => {
      trigger.focus();
      panel.hidePopover();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (!items.length) return;
      if (event.key === "Tab") {
        close();
        return;
      }
      const current = items.findIndex(
        (item) => item === document.activeElement,
      );
      let index: number;
      switch (event.key) {
        case "ArrowDown":
          if (nativeFocusgroup) return;
          index = (current + 1) % items.length;
          break;
        case "ArrowUp":
          if (nativeFocusgroup) return;
          index = (current + items.length - 1) % items.length;
          break;
        case "Home":
          index = 0;
          break;
        case "End":
          index = items.length - 1;
          break;
        default:
          return;
      }
      event.preventDefault();
      focus(items[index]);
    };
    const onFocus = (event: FocusEvent) => {
      if (nativeFocusgroup) return;
      items.forEach((item) => {
        item.tabIndex = item === event.target ? 0 : -1;
      });
    };
    const onOutsideFocus = (event: FocusEvent) => {
      if (
        event.target instanceof Node &&
        !panel.contains(event.target) &&
        event.target !== trigger
      )
        panel.hidePopover();
    };
    const onPointerMove = (event: PointerEvent) => {
      if (!(event.target instanceof Element)) return;
      const item = event.target.closest<HTMLButtonElement>("button");
      if (item && items.includes(item) && item !== document.activeElement)
        focus(item);
    };
    const position = () => {
      if (CSS.supports("position-area: block-end")) return;
      panel.classList.add("oui-platform-menu-positioned");
      const anchor = trigger.getBoundingClientRect();
      const bounds = panel.getBoundingClientRect();
      const gutter = parseFloat(getComputedStyle(panel).paddingInlineStart);
      const left = Math.max(
        gutter,
        Math.min(
          anchor.right - bounds.width,
          window.innerWidth - bounds.width - gutter,
        ),
      );
      const top =
        anchor.bottom + bounds.height + gutter <= window.innerHeight
          ? anchor.bottom
          : Math.max(gutter, anchor.top - bounds.height);
      panel.style.left = `${left}px`;
      panel.style.top = `${top}px`;
    };
    position();
    group.addEventListener("keydown", onKeyDown);
    group.addEventListener("focusin", onFocus);
    group.addEventListener("pointermove", onPointerMove);
    group.addEventListener("click", close);
    document.addEventListener("focusin", onOutsideFocus);
    window.addEventListener("resize", position);
    window.addEventListener("scroll", position, true);
    return () => {
      group.removeEventListener("keydown", onKeyDown);
      group.removeEventListener("focusin", onFocus);
      group.removeEventListener("pointermove", onPointerMove);
      group.removeEventListener("click", close);
      document.removeEventListener("focusin", onOutsideFocus);
      window.removeEventListener("resize", position);
      window.removeEventListener("scroll", position, true);
    };
  }, []);
  return (
    <div
      ref={ref}
      {...{ focusgroup: "menu nomemory" }}
      aria-label={props.label}
      className="oui-flex oui-flex-col oui-gap-2"
    >
      {props.children}
    </div>
  );
}
