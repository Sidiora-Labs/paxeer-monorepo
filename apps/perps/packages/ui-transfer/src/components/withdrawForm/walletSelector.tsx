import { FC, ReactNode, SVGProps, useMemo, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  cn,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  Flex,
  Text,
  WalletIcon,
  CaretDownIcon,
  CaretUpIcon,
  useDocumentDirection,
} from "@paxeer-network/ui";

const AddIcon: FC<SVGProps<SVGSVGElement>> = (props) => (
  <svg
    width="18"
    height="18"
    viewBox="0 0 18 18"
    fill="currentColor"
    xmlns="http://www.w3.org/2000/svg"
    {...props}
  >
    <path d="M8.99316 2.19731C8.57916 2.19731 8.24316 2.53331 8.24316 2.94731V8.19731H2.99316C2.57916 8.19731 2.24316 8.53331 2.24316 8.94731C2.24316 9.36131 2.57916 9.69731 2.99316 9.69731H8.24316V14.9473C8.24316 15.3613 8.57916 15.6973 8.99316 15.6973C9.40716 15.6973 9.74316 15.3613 9.74316 14.9473V9.69731H14.9932C15.4072 9.69731 15.7432 9.36131 15.7432 8.94731C15.7432 8.53331 15.4072 8.19731 14.9932 8.19731H9.74316V2.94731C9.74316 2.53331 9.40716 2.19731 8.99316 2.19731Z" />
  </svg>
);

export type WalletOption = {
  address: string;
  name?: string;
  network?: "EVM" | "SOL";
};

interface WalletMenuItemProps {
  onClick: () => void;
  children: ReactNode;
  className?: string;
}

const WalletMenuItem: FC<WalletMenuItemProps> = ({
  onClick,
  children,
  className,
}) => {
  return (
    <DropdownMenuItem asChild>
      <button
        type="button"
        onClick={onClick}
        className={cn(
          "oui-flex oui-items-center oui-gap-2 oui-py-1.5 oui-px-2 hover:oui-bg-base-5",
          "oui-rounded",
          "oui-font-semibold oui-tracking-wide",
          className,
        )}
      >
        {children}
      </button>
    </DropdownMenuItem>
  );
};

interface WalletSelectorProps {
  connectedWallet?: {
    name: string;
    address: string;
    namespace?: string;
  };
  externalWallets: WalletOption[];
  selectedAddress: string;
  onSelect: (address: string) => void;
  onAddExternalWallet: () => void;
}

export const WalletSelector: FC<WalletSelectorProps> = ({
  connectedWallet,
  externalWallets,
  selectedAddress,
  onSelect,
  onAddExternalWallet,
}) => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const dir = useDocumentDirection();
  const isRTL = dir === "rtl";

  const currentNetwork = useMemo<"EVM" | "SOL" | undefined>(() => {
    if (!connectedWallet?.namespace) return undefined;
    const ns = connectedWallet.namespace.toLowerCase();
    if (ns.includes("sol")) return "SOL";
    return "EVM";
  }, [connectedWallet?.namespace]);

  const filteredExternalWallets = useMemo(() => {
    if (!currentNetwork) return externalWallets;
    return externalWallets.filter(
      (wallet) => wallet.network === currentNetwork,
    );
  }, [externalWallets, currentNetwork]);

  const selectedWalletOpt = useMemo(() => {
    if (connectedWallet && selectedAddress === connectedWallet.address) {
      return {
        address: connectedWallet.address,
        name: connectedWallet.name,
        network: currentNetwork,
      };
    }
    return filteredExternalWallets.find(
      (w: WalletOption) => w.address === selectedAddress,
    );
  }, [
    connectedWallet,
    currentNetwork,
    filteredExternalWallets,
    selectedAddress,
  ]);

  const hasExternalWallets = filteredExternalWallets.length > 0;
  const showConnectedItem = !!connectedWallet && hasExternalWallets;

  const getChainLabel = (network?: "EVM" | "SOL") => {
    if (!network) return "";
    if (network === "SOL") return "Solana";
    return "EVM";
  };

  const handleSelectWallet = (address: string) => {
    onSelect(address);
    setIsOpen(false);
  };

  const handleAddExternalWalletClick = () => {
    onAddExternalWallet();
    setIsOpen(false);
  };

  const dropdownContent = (
    <DropdownMenuPortal>
      <DropdownMenuContent
        align={isRTL ? "start" : "end"}
        className="oui-max-h-60 oui-max-w-[calc(100vw-var(--oui-space-6))] oui-overflow-y-auto [&_[role=menuitem]]:oui-min-h-12 [&_[role=menuitem]]:oui-h-auto [&_[role=menuitem]]:oui-whitespace-normal [&_[role=menuitem]]:oui-break-all"
      >
        {showConnectedItem && connectedWallet && (
          <WalletMenuItem
            onClick={() => handleSelectWallet(connectedWallet.address)}
            className="oui-w-96"
          >
            <Flex direction="column" itemAlign="start" gapY={1}>
              <Text size="2xs" className="oui-text-base-contrast-80">
                {t("common.myWallet")}
              </Text>
              <Text
                size="2xs"
                intensity={80}
                className="oui-text-base-contrast-80 oui-leading-normal"
              >
                {`(${getChainLabel(currentNetwork)}) ${connectedWallet.address}`}
              </Text>
            </Flex>
          </WalletMenuItem>
        )}

        {hasExternalWallets && (
          <>
            {filteredExternalWallets.map((wallet) => (
              <WalletMenuItem
                key={wallet.address}
                onClick={() => handleSelectWallet(wallet.address)}
                className="oui-w-96"
              >
                <Flex direction="column" itemAlign="start" gapY={1}>
                  <Text size="2xs" className="oui-text-base-contrast-80">
                    {t("common.externalWallet")}
                  </Text>
                  <Text
                    size="2xs"
                    intensity={80}
                    className="oui-text-base-contrast-80 oui-leading-normal"
                  >
                    {`(${getChainLabel(wallet.network)}) ${wallet.address}`}
                  </Text>
                </Flex>
              </WalletMenuItem>
            ))}
          </>
        )}

        <WalletMenuItem
          onClick={handleAddExternalWalletClick}
          className={cn(
            "oui-justify-center",
            hasExternalWallets && "oui-w-full",
            !hasExternalWallets && "oui-w-48",
          )}
        >
          <Flex
            direction={isRTL ? "rowReverse" : "row"}
            itemAlign="center"
            gapX={1}
            className="oui-text-primary oui-font-semibold oui-tracking-wide"
          >
            <AddIcon className="oui-size-4" />
            <Text size="2xs">{t("transfer.withdraw.addExternalWallet")}</Text>
          </Flex>
        </WalletMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  );

  return (
    <Flex
      justify="between"
      className="oui-w-full oui-mb-3 oui-flex-wrap oui-gap-2"
    >
      <Text
        size="2xs"
        intensity={80}
        className="oui-shrink-0 oui-whitespace-nowrap"
      >
        {t("common.wallet")}
      </Text>
      <DropdownMenuRoot open={isOpen} onOpenChange={setIsOpen}>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t("common.wallet")}
            className={cn(
              "oui-flex oui-min-w-0 oui-flex-wrap oui-items-center oui-gap-1 oui-cursor-pointer oui-text-base-contrast-80 hover:oui-text-base-contrast-80",
              isRTL && "oui-flex-row-reverse",
            )}
          >
            {selectedWalletOpt?.name && (
              <WalletIcon name={selectedWalletOpt.name} size="3xs" />
            )}
            <Text.formatted
              size="2xs"
              intensity={80}
              rule="address"
              className="oui-whitespace-nowrap"
            >
              {selectedWalletOpt?.address || ""}
            </Text.formatted>
            <Text
              size="2xs"
              intensity={80}
              className="oui-text-base-contrast-80"
            >
              {` (${getChainLabel(selectedWalletOpt?.network)})`}
            </Text>
            {isOpen ? (
              <CaretUpIcon size={12} className="oui-text-inherit" />
            ) : (
              <CaretDownIcon size={12} className="oui-text-inherit" />
            )}
          </button>
        </DropdownMenuTrigger>
        {dropdownContent}
      </DropdownMenuRoot>
    </Flex>
  );
};
