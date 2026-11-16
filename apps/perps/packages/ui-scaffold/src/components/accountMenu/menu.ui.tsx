import { useState } from "react";
import {
  Menu,
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
} from "../../../../ui-connector/src/platformOverlays";
import { useTranslation } from "@paxeer-network/i18n";
import { AccountStatusEnum } from "@paxeer-network/types";
import { Button, EVMAvatar, Flex, Text } from "@paxeer-network/ui";

export type AccountMenuProps = {
  accountState: AccountState;
  address?: string;
  onDisconnect: () => void;
  connect: () => Promise<void>;
  onCrateAccount: () => Promise<void>;
  onCreatePaxeerNetworkKey: () => Promise<void>;
  onOpenExplorer: () => void;
  onSwitchNetwork: () => void;
  wrongNetwork: boolean;
  disabledConnect?: boolean;
  isMobile: boolean;
};

export const AccountMenu = (props: AccountMenuProps) => {
  const { t } = useTranslation();
  const {
    accountState: state,
    onDisconnect,
    onOpenExplorer,
    wrongNetwork,
    onSwitchNetwork,
    isMobile,
  } = props;
  const disabled = state.validating || props.disabledConnect;

  if (!disabled && wrongNetwork) {
    return (
      <Button
        data-testid="oui-testid-nav-bar-wrongNetwork-btn"
        size="md"
        color="warning"
        className="oui-shell-action"
        onClick={onSwitchNetwork}
      >
        {t("connector.wrongNetwork")}
      </Button>
    );
  }

  if (
    !disabled &&
    (state.status === AccountStatusEnum.EnableTrading ||
      state.status === AccountStatusEnum.EnableTradingWithoutConnected)
  ) {
    return (
      <WalletMenu
        address={state.address!}
        onDisconnect={onDisconnect}
        onOpenExplorer={onOpenExplorer}
      />
    );
  }

  if (state.status <= AccountStatusEnum.NotConnected || disabled) {
    return (
      <Button
        data-testid="oui-testid-nav-bar-connectWallet-btn"
        size="md"
        className="wallet-connect-button oui-shell-action"
        loading={state.validating}
        disabled={disabled}
        onClick={() => {
          props
            .connect()
            .then((r) => {
              console.log("*****", r);
            })
            .catch((e) => console.error(e));
        }}
      >
        {isMobile ? t("connector.connect") : t("connector.connectWallet")}
      </Button>
    );
    // return (
    //   <Tooltip
    //     open
    //     content={"Please connect wallet before starting to trade"}
    //     align={"end"}
    //     className="oui-bg-base-5"
    //     arrow={{ className: "oui-fill-base-5" }}
    //   >

    //   </Tooltip>
    // );
  }

  if (state.status <= AccountStatusEnum.NotSignedIn) {
    return (
      <Button
        size="md"
        className="oui-shell-action"
        onClick={() => props.onCrateAccount()}
      >
        {t("connector.createAccount")}
      </Button>
    );
    // return (
    //   <Tooltip
    //     open
    //     content={"Please sign in before starting to trade"}
    //     align={"end"}
    //     className="oui-bg-base-5"
    //     arrow={{ className: "oui-fill-base-5" }}
    //   >

    //   </Tooltip>
    // );
  }

  if (state.status <= AccountStatusEnum.DisabledTrading) {
    return (
      <Button
        size="md"
        className="oui-shell-action"
        onClick={() => {
          props
            .onCreatePaxeerNetworkKey()
            .then((r) => console.log(r))
            .catch((e) => console.error(e));
        }}
      >
        {t("connector.enableTrading")}
      </Button>
    );
    // return (
    //   <Tooltip
    //     open
    //     className="oui-bg-base-5"
    //     arrow={{ className: "oui-fill-base-5" }}
    //     content={"Please enable trading before starting to trade"}
    //     align={"end"}
    //   >

    //   </Tooltip>
    // );
  }
};

export type AccountState = {
  status: AccountStatusEnum;

  /**
   * whether the account is validating
   */
  validating: boolean;
  chainNamespace?: string;

  accountId?: string;
  userId?: string;
  address?: string;
  isNew?: boolean;

  connectWallet?: {
    name: string;
    chainId: number;
  };
};

const WalletMenu = (props: {
  address: string;
  onDisconnect: () => void;
  onOpenExplorer: () => void;
}) => {
  const { address, onDisconnect } = props;
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <PopoverRoot open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          size="md"
          variant="contained"
          className="oui-shell-action"
          aria-haspopup="menu"
          data-testid="oui-testid-nav-bar-address-btn"
        >
          <Text.formatted rule="address" className="oui-text-primary-contrast">
            {address}
          </Text.formatted>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="oui-shell-overlay oui-p-4 oui-font-semibold"
      >
        <Flex gap={2} className="oui-mb-4 oui-min-w-0">
          <EVMAvatar address={address} size="md" />
          <Text.formatted rule="address" size="base">
            {address}
          </Text.formatted>
        </Flex>
        <Menu label="Wallet actions">
          <button
            type="button"
            className="oui-shell-action oui-px-3 oui-text-start"
            data-testid="oui-testid-nav-bar-dropDownMenuItem-copyAddress"
            onClick={() => navigator.clipboard.writeText(address)}
          >
            {t("common.copy")}
          </button>
          <button
            type="button"
            className="oui-shell-action oui-px-3 oui-text-start"
            onClick={props.onOpenExplorer}
          >
            Open address in explorer
          </button>
          <button
            type="button"
            className="oui-shell-action oui-px-3 oui-text-start oui-text-base-contrast"
            data-testid="oui-testid-nav-bar-dropDownMenuItem-disconnect"
            onClick={() => {
              onDisconnect();
              setOpen(false);
            }}
          >
            {t("connector.disconnect")}
          </button>
        </Menu>
      </PopoverContent>
    </PopoverRoot>
  );
};
