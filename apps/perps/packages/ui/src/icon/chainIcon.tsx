import { useMemo, type FC } from "react";
import { Avatar } from "../avatar";

export type ChainIconProps = {
  size?: "2xs" | "xs" | "sm" | "md" | "lg";
  chainId: string | number;
  className?: string;
};

export const ChainIcon: FC<ChainIconProps> = (props) => {
  const url = useMemo(() => {
    return String(props.chainId) === "125"
      ? "/paxeer-network-symbol.svg"
      : `/assets/static/network_logo/${props.chainId}.png`;
  }, [props.chainId]);
  return (
    <Avatar
      size={props.size}
      src={url}
      alt={`${props.chainId}`}
      className={props.className}
    />
  );
};
