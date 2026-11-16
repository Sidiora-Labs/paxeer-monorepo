import * as React from "react";
import {
  Button,
  type ButtonProps,
} from "@paxeer-network/paxeer-backend/ui/atoms/button";
import type { ToastProps } from "@paxeer-network/paxeer-backend/ui/atoms/use-toast";

export type ToastActionElement = NonNullable<ToastProps["action"]>;

export interface ToastActionProps extends Omit<ButtonProps, "asChild"> {
  altText: string;
}

export const ToastAction = React.forwardRef<
  HTMLButtonElement,
  ToastActionProps
>(({ altText, variant = "outline", size = "sm", ...props }, ref) => (
  <Button
    {...props}
    ref={ref}
    type="button"
    variant={variant}
    size={size}
    aria-label={altText}
  />
));
ToastAction.displayName = "ToastAction";
