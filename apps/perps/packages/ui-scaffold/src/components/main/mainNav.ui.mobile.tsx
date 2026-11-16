import { FC, useMemo } from "react";
import { useAccount } from "@paxeer-network/hooks";
import { useAppContext } from "@paxeer-network/react-app";
import { AccountStatusEnum, type RouterAdapter } from "@paxeer-network/types";
import { Flex, Text, ChevronLeftIcon, cn } from "@paxeer-network/ui";
import { WalletConnectButtonExtension } from "../accountMenu/menu.widget";
import { ChainMenuWidget } from "../chainMenu";
import { LanguageSwitcherWidget } from "../languageSwitcher";
import { LeftNavWidget } from "../leftNav";
import { ScanQRCodeWidget } from "../scanQRCode";
import { SubAccountWidget } from "../subAccount";
import { LinkDeviceWidget } from "./linkDevice";
import { MainLogo } from "./mainLogo";
import { MainNavWidgetProps } from "./mainNav.widget";

type MainNavMobileProps = {
  current?: string;
  subItems?: {
    name: string;
    href: string;
  }[];
  routerAdapter?: RouterAdapter;
} & MainNavWidgetProps;

export const MainNavMobile: FC<MainNavMobileProps> = (props) => {
  const { wrongNetwork, disabledConnect } = useAppContext();
  const { state } = useAccount();
  const currentMenu = useMemo(() => {
    if (Array.isArray(props?.initialMenu)) {
      return props?.campaigns;
    }
    return props?.mainMenus?.find((menu) => {
      if (!props.current) {
        return menu.href === props?.initialMenu;
      } else {
        return menu.href === props.current;
      }
    });
  }, [props?.mainMenus, props?.initialMenu, props?.current]);

  const title = useMemo(() => {
    if (currentMenu?.isHomePageInMobile) {
      return <MainLogo {...props?.logo} />;
    }
    return (
      <Text className="oui-text-2xl oui-font-bold oui-text-base-contrast">
        {currentMenu?.name}
      </Text>
    );
  }, [currentMenu, props?.logo]);

  const isSub = useMemo(() => {
    return Boolean(!currentMenu || currentMenu.isSubMenuInMobile);
  }, [currentMenu]);

  const subTitle = useMemo(() => {
    if (currentMenu?.isSubMenuInMobile) {
      return currentMenu?.name;
    }
    if (props?.subItems?.some((item) => item.href === props?.current)) {
      return props?.subItems?.find((item) => item.href === props?.current)
        ?.name;
    }
    return null;
  }, [props?.subItems, props?.current, currentMenu]);

  const onBack = () => {
    let target = props.mainMenus?.find(
      (item) => item.href === props.initialMenu,
    );
    if (currentMenu?.isSubMenuInMobile) {
      target = currentMenu?.subMenuBackNav;
    }

    if (target && typeof window !== "undefined") {
      const url = new URL(window.location.href);
      const cleanUrl = `${url.pathname}`;
      window.history.replaceState({}, "", cleanUrl);
    }

    props?.routerAdapter?.onRouteChange(target as any);
  };

  const showLinkDevice =
    state.status === AccountStatusEnum.EnableTradingWithoutConnected;

  const showChainMenu = !showLinkDevice && !wrongNetwork;

  const showQrcode = state.status === AccountStatusEnum.NotConnected;

  const showSubAccount = useMemo(
    () => state.status >= AccountStatusEnum.EnableTrading,
    [state.status],
  );

  if (isSub) {
    return (
      <Flex
        width={"100%"}
        px={3}
        direction={"row"}
        itemAlign={"center"}
        justify={"center"}
        className="oui-relative oui-min-h-12"
      >
        <button
          type="button"
          aria-label="Back"
          className="oui-shell-icon-button oui-absolute oui-start-0"
          onClick={onBack}
        >
          <ChevronLeftIcon aria-hidden="true" />
        </button>
        <Text className="oui-text-base oui-font-bold oui-text-base-contrast">
          {subTitle}
        </Text>
      </Flex>
    );
  }

  const renderContent = () => {
    const languageSwitcher = <LanguageSwitcherWidget />;
    const scanQRCode = showQrcode && <ScanQRCodeWidget />;
    const subAccount = showSubAccount && <SubAccountWidget />;
    const linkDevice = showLinkDevice && <LinkDeviceWidget />;
    const chainMenu = showChainMenu && <ChainMenuWidget />;
    const walletConnect = <WalletConnectButtonExtension />;

    const leftNav = props.leftNav && (
      <LeftNavWidget
        {...props.leftNav}
        logo={props?.logo}
        routerAdapter={props?.routerAdapter}
      />
    );

    if (typeof props.customRender === "function") {
      return props.customRender?.({
        leftNav,
        title,
        languageSwitcher,
        scanQRCode,
        subAccount,
        linkDevice,
        chainMenu,
        walletConnect,
      });
    }

    return (
      <Flex width="100%" justify="between" className="oui-flex-wrap oui-gap-2">
        <Flex gapX={2} className="oui-flex-wrap oui-min-w-0">
          {props?.customLeftNav || leftNav}
          {title}
        </Flex>

        <Flex gapX={2} className="oui-flex-wrap oui-min-w-0">
          {props.leading}
          {languageSwitcher}
          {scanQRCode}
          {/* {subAccount} */}
          {/* {linkDevice} */}
          {chainMenu}
          {walletConnect}
          {props.trailing}
        </Flex>
      </Flex>
    );
  };

  return (
    <Flex
      width={"100%"}
      px={3}
      itemAlign={"center"}
      className={cn(
        "oui-min-h-12 oui-py-2",
        props.className,
        props.classNames?.root,
      )}
    >
      {renderContent()}
    </Flex>
  );
};
