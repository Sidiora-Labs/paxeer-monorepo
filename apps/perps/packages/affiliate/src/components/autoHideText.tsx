import { FC } from "react";
import { cn } from "@paxeer-network/ui";

export const AutoHideText: FC<{
  text: string;
  className?: string;
  visibleCount?: number;
}> = ({ text, className }) => (
  <span className={cn("oui-min-w-0 oui-break-all oui-text-end", className)}>
    {text}
  </span>
);
