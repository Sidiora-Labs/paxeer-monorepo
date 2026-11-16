import { FC } from "react";
import { ShareIcon } from "@paxeer-network/ui";
import { ShareButtonScriptReturn } from "./shareButton.script";

export const ShareButton: FC<ShareButtonScriptReturn> = (props) => {
  if (props.sharePnLConfig == null) {
    return null;
  }

  return (
    <button
      type="button"
      aria-label="Share profit and loss"
      className="oui-touch-target oui-inline-flex oui-items-center oui-justify-center oui-rounded-full focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary"
      onClick={(e: any) => {
        e.stopPropagation();
        props.showModal();
      }}
    >
      <ShareIcon color="white" opacity={0.54} size={props.iconSize ?? 16} />
    </button>
  );
};
