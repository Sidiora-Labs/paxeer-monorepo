import { useId } from "react";
import { cn } from "@paxeer-network/ui";

export const Checkbox = (props: {
  size?: number;
  className?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label?: string;
}) => {
  const id = useId();
  return (
    <input
      id={id}
      type="checkbox"
      aria-label={props.label}
      checked={props.checked}
      onChange={(event) => props.onCheckedChange(event.target.checked)}
      className={cn(
        "oui-size-5 oui-shrink-0 oui-accent-primary",
        props.className,
      )}
    />
  );
};
