import { FC, PropsWithChildren, useMemo } from "react";
import { AccountStatusEnum } from "@paxeer-network/types";
import { cn, Divider, Flex, useScreen } from "@paxeer-network/ui";
import { WalletConnectButtonExtension } from "../accountMenu/menu.widget";
import { AccountSummaryWidget } from "../accountSummary";
import { ChainMenuWidget } from "../chainMenu";
import { LanguageSwitcherWidget } from "../languageSwitcher";
import { MessageCenterWidget } from "../messageCenter/msgCenter.widget";
import { SubAccountWidget } from "../subAccount";
import { CampaignButton } from "./campaignButton";
import { LinkDeviceWidget } from "./linkDevice";
import { MainLogo } from "./mainLogo";
import { MainNavMenusExtension } from "./mainMenus/mainNavMenus.widget";
import { CampaignPositionEnum, MainNavScriptReturn } from "./mainNav.script";

export const MainNav: FC<PropsWithChildren<MainNavScriptReturn>> = (props) => {
  const { className, classNames, campaigns, campaignPosition } = props;

  const showCampaignButton =
    campaignPosition === CampaignPositionEnum.navTailing && campaigns;

  const showLinkIcon =
    !props.wrongNetwork &&
    !props.disabledConnect &&
    props.status! >= AccountStatusEnum.SignedIn;

  const showSubAccount = props.status! >= AccountStatusEnum.EnableTrading;

  const hideWalletConnectButton =
    !props.disabledConnect && props.wrongNetwork && props.isConnected;

  const { isDesktop } = useScreen();

  const children = useMemo(() => {
    if (typeof props.children === "undefined") {
      return null;
    }
    return <Flex grow>{props.children}</Flex>;
  }, [props.children]);

  const showChainMenu = isDesktop;

  const renderContent = () => {
    const title = <MainLogo {...props.logo} />;
    const accountSummary = <AccountSummaryWidget />;
    const linkDevice = showLinkIcon && <LinkDeviceWidget />;
    const languageSwitcher = <LanguageSwitcherWidget />;
    const subAccount = showSubAccount && <SubAccountWidget />;
    const chainMenu = showChainMenu && <ChainMenuWidget />;
    const notify = <MessageCenterWidget />;
    const walletConnect = !hideWalletConnectButton && (
      <WalletConnectButtonExtension />
    );

    const mainNav = (
      <>
        <MainNavMenusExtension
          {...props.mainMenus}
          classNames={classNames?.mainNav}
        />
        {!!showCampaignButton && (
          <CampaignButton
            {...campaigns}
            className={classNames?.campaignButton}
          />
        )}
      </>
    );

    if (typeof props.customRender === "function") {
      return props.customRender?.({
        title,
        mainNav,
        accountSummary,
        linkDevice,
        languageSwitcher,
        subAccount,
        chainMenu,
        notify,
        walletConnect,
      });
    }

    return (
      <>
        <Flex
          itemAlign={"center"}
          className={cn(
            "oui-gap-3",
            // let the left and right views show spacing when overlapping
            "oui-min-w-0 oui-flex-wrap",
          )}
        >
          {title}
          {props.leading}
          <Flex
            className="oui-gap-3 oui-flex-wrap oui-min-w-0"
            style={{ scrollbarWidth: "none" }}
          >
            {mainNav}
          </Flex>
          {props.trailing}
        </Flex>

        {children}

        <Flex itemAlign={"center"} className="oui-gap-2 oui-flex-wrap">
          {accountSummary}
          {showLinkIcon && (
            <>
              <Divider direction="vertical" className="oui-h-8" intensity={8} />
              {linkDevice}
            </>
          )}
          {notify}
          {languageSwitcher}
          {subAccount}
          {chainMenu}
          {walletConnect}
        </Flex>
      </>
    );
  };

  return (
    <Flex
      width="100%"
      as="header"
      itemAlign={"center"}
      justify={"between"}
      px={3}
      gapX={3}
      className={cn(
        "oui-main-nav oui-min-h-12 oui-flex-wrap oui-py-2 oui-font-semibold oui-border-0 lg:oui-border-line-12 lg:oui-border-b",
        className,
        classNames?.root,
      )}
    >
      {renderContent()}
    </Flex>
  );
};

MainNav.displayName = "MainNav";
