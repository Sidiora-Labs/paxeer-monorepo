import { useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { API } from "@paxeer-network/types";
import {
  Badge,
  Box,
  ChainIcon,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  Flex,
  ScrollArea,
  Spinner,
  Text,
  cn,
  useDocumentDirection,
} from "@paxeer-network/ui";
import { ExchangeIcon } from "../../icons";
import { CurrentChain } from "../../types";

type ChainSelectProps = {
  chains: API.NetworkInfos[];
  value: CurrentChain;
  onValueChange: (chain: API.NetworkInfos) => Promise<void>;
  wrongNetwork: boolean;
  loading?: boolean;
  disabled?: boolean;
};

export const ChainSelect: React.FC<ChainSelectProps> = (props) => {
  const { chains, value, wrongNetwork, loading, disabled } = props;
  const [open, setOpen] = useState(false);
  const { t } = useTranslation();
  const dir = useDocumentDirection();
  const isRTL = dir === "rtl";

  const selectable = wrongNetwork || chains?.length > 1;

  const chainIcon = wrongNetwork ? (
    <Flex
      width="var(--oui-space-4)"
      height="var(--oui-space-4)"
      intensity={100}
      r="full"
      justify="center"
      itemAlign="center"
    >
      <Text size="2xs" intensity={80}>
        U
      </Text>
    </Flex>
  ) : (
    <ChainIcon className="oui-size-4" chainId={value?.id} />
  );

  const chainName = wrongNetwork ? "Unknown" : value?.info?.network_infos?.name;

  const currentChain = chains.find((chain) => chain.chain_id === value?.id);
  const extendedCurrentChain = currentChain as API.NetworkInfos & {
    isSupported?: boolean;
  };
  const isCurrentChainSupported =
    !currentChain || extendedCurrentChain?.isSupported !== false;

  const renderRightIcon = () => {
    if (loading) {
      return <Spinner size="sm" />;
    }
    if (selectable) {
      return <ExchangeIcon className="oui-text-base-contrast-80" />;
    }
  };

  const trigger = (
    <Flex
      asChild
      className={cn(
        "oui-rounded-t-xl oui-rounded-b-sm oui-bg-base-6",
        disabled
          ? "oui-cursor-not-allowed"
          : selectable
            ? "oui-cursor-pointer"
            : "oui-cursor-auto",
      )}
      height="var(--oui-space-12)"
      px={3}
      justify="between"
      itemAlign="center"
    >
      <button
        type="button"
        disabled={disabled || !selectable}
        className="oui-w-full oui-text-start oui-min-h-12"
      >
        <div>
          <Flex>
            <Text size="2xs" intensity={80}>
              {t("transfer.network")}
            </Text>
          </Flex>
          <Flex
            direction={isRTL ? "rowReverse" : "row"}
            gapX={1}
            itemAlign="center"
          >
            {chainIcon}
            <Text size="sm" intensity={80}>
              {chainName}
            </Text>
            {!isCurrentChainSupported && (
              <Badge color="danger" size="xs">
                {t("common.notSupported")}
              </Badge>
            )}
          </Flex>
        </div>
        {renderRightIcon()}
      </button>
    </Flex>
  );

  const content = chains.map((chain, index) => {
    const extendedChain = chain as API.NetworkInfos & {
      isSupported?: boolean;
    };
    const isActive = chain.chain_id === value?.id;
    const isSupported = extendedChain.isSupported !== false;
    return (
      <DropdownMenuItem asChild key={chain.chain_id} disabled={!isSupported}>
        <Flex
          asChild
          key={chain.chain_id}
          px={2}
          r="base"
          direction={isRTL ? "rowReverse" : "row"}
          justify="between"
          itemAlign="center"
          className={cn(
            "oui-deposit-network-select-item",
            "oui-h-7",
            isSupported
              ? "hover:oui-bg-base-5 oui-cursor-pointer"
              : "oui-cursor-not-allowed",
            isActive && "oui-bg-base-5",
            index !== 0 && "oui-mt-0.5",
          )}
        >
          <button
            type="button"
            disabled={!isSupported}
            onClick={async () => {
              if (!isSupported) return;
              setOpen(false);
              await props.onValueChange(chain);
            }}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
          >
            <Flex
              direction={isRTL ? "rowReverse" : "row"}
              gapX={1}
              itemAlign="center"
            >
              <ChainIcon
                className={cn("oui-size-4", !isSupported && "oui-opacity-50")}
                chainId={chain.chain_id}
              />
              <Text size="2xs" intensity={80}>
                {chain.name}
              </Text>
              {!isSupported && (
                <Badge color="danger" size="xs">
                  {t("common.notSupported")}
                </Badge>
              )}
            </Flex>
            {isActive && (
              <Box
                width="var(--oui-space-1)"
                height="var(--oui-space-1)"
                r="full"
                className="oui-deposit-network-select-active-dot oui-gradient-brand"
              />
            )}
          </button>
        </Flex>
      </DropdownMenuItem>
    );
  });

  return (
    <DropdownMenuRoot open={selectable ? open : false} onOpenChange={setOpen}>
      <DropdownMenuTrigger disabled={disabled} asChild>
        {trigger}
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent
          align={isRTL ? "end" : "start"}
          sideOffset={4}
          className={cn(
            "oui-deposit-token-select-dropdown-menu-content",
            "oui-bg-base-8 oui-p-1",
            "oui-w-[var(--radix-dropdown-menu-trigger-width)] oui-max-w-[calc(100vw-var(--oui-space-6))] [&_[role=menuitem]]:oui-min-h-12 [&_[role=menuitem]]:oui-h-auto",
            "oui-rounded-md oui-select-none",
          )}
        >
          <ScrollArea>
            <div className="oui-max-h-64">{content}</div>
          </ScrollArea>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  );
};
