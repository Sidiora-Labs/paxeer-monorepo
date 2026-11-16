import { Tooltip } from "../../../../ui-connector/src/platformOverlays";
import {
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
} from "../../../../ui-connector/src/platformOverlays";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, ChainIcon, cn, modal, Spinner } from "@paxeer-network/ui";
import {
  ChainSelectorDialogId,
  ChainSelectorWidget,
} from "@paxeer-network/ui-chain-selector";
import { UseChainMenuScriptReturn } from "./chainMenu.script";

export const ChainMenu = (props: UseChainMenuScriptReturn) => {
  const { t } = useTranslation();

  if (props.wrongNetwork && props.isConnected && !props.disabledConnect) {
    return (
      <Tooltip
        open
        hideWhenDetached
        content={t("connector.wrongNetwork.tooltip")}
        className="oui-bg-base-5"
        arrow={{ className: "oui-fill-base-5" }}
      >
        <Button
          color="warning"
          size="md"
          onClick={() => {
            modal
              .show<{ wrongNetwork: boolean }>(ChainSelectorDialogId, {
                networkId: props.networkId,
                onChainChangeBefore: props.onChainChangeBefore,
                onChainChangeAfter: props.onChainChangeAfter,
              })
              .catch((error) => console.log(error));
          }}
        >
          {t("connector.wrongNetwork")}
        </Button>
      </Tooltip>
    );
  }

  const renderIcon = () => {
    if (props.loading) {
      return <Spinner className="oui-w-5 oui-h-5" />;
    }

    if (props.currentChainId) {
      return <ChainIcon chainId={props.currentChainId} size="xs" />;
    }
  };

  const trigger = (
    <button
      type="button"
      aria-label={t("connector.switchNetwork")}
      className={cn(
        "oui-relative oui-shell-icon-button oui-bg-base-5",

        "oui-rounded-t-md oui-rounded-bl-md oui-rounded-br-sm",
      )}
    >
      {renderIcon()}
      <svg
        width="10"
        height="10"
        viewBox="0 0 10 10"
        xmlns="http://www.w3.org/2000/svg"
        className="oui-absolute oui-end-0 oui-bottom-0"
      >
        <defs>
          <linearGradient
            id="paint0_linear_490_5080"
            x1="10"
            y1="5"
            x2="-5.79673e-08"
            y2="5"
            gradientUnits="userSpaceOnUse"
          >
            <stop stopColor="rgb(var(--oui-gradient-brand-end))" />
            <stop offset="1" stopColor="rgb(var(--oui-gradient-brand-start))" />
          </linearGradient>
        </defs>
        <path
          d="M10 7V0L0 10H7C8.65685 10 10 8.65685 10 7Z"
          fill="url(#paint0_linear_490_5080)"
        />
      </svg>
    </button>
  );

  return (
    <PopoverRoot open={props.open} onOpenChange={props.onOpenChange}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent
        aria-label={t("connector.switchNetwork")}
        onClick={(e) => e.stopPropagation()}
        sideOffset={4}
        collisionPadding={{ left: 16, right: 16 }}
        className={cn(
          "oui-shell-overlay oui-bg-base-8 oui-w-96 oui-p-4 oui-rounded-xl",
          "oui-border oui-border-line-6",
          "oui-font-semibold",
        )}
      >
        <ChainSelectorWidget
          close={props.hide}
          onChainChangeBefore={props.onChainChangeBefore}
          onChainChangeAfter={props.onChainChangeAfter}
        />
      </PopoverContent>
    </PopoverRoot>
  );
};
