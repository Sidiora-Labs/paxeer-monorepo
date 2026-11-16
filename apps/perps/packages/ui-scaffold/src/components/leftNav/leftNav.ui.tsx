import { SimpleDialog } from "../../../../ui-connector/src/platformOverlays";
import { FC, useMemo } from "react";
import { useAccount } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import {
  AccountStatusEnum,
  type RouterAdapter,
  type RouteOption,
} from "@paxeer-network/types";
import {
  VectorIcon,
  SwapHorizIcon,
  PeopleIcon,
  Text,
  Divider,
} from "@paxeer-network/ui";
import { MainLogo } from "../main/mainLogo";
import { DefaultTradingViewTncLink } from "../scaffold/scaffold.ui";
import { SubAccountWidget } from "../subAccount";
import {
  CommunityDiscord,
  CommunityDune,
  CommunityTG,
  CommunityX,
} from "./communityIcon";
import { LeftNavScriptReturn } from "./leftNav.script";
import { LeftNavItem, LeftNavProps } from "./leftNav.type";

export type LeftNavUIProps = LeftNavProps &
  Partial<LeftNavScriptReturn> & {
    className?: string;
    logo?: {
      src: string;
      alt: string;
    };
    routerAdapter?: RouterAdapter;
    showSubAccount?: boolean;
  };

export const LeftNavUI: FC<LeftNavUIProps> = (props) => {
  return (
    <>
      <button
        type="button"
        aria-label="Navigation"
        onClick={props.showSheet}
        className={`oui-shell-icon-button ${props.className ?? ""}`}
      >
        <VectorIcon aria-hidden="true" />
      </button>
      <LeftNavSheet {...props} />
    </>
  );
};

const LeftNavSheet: FC<LeftNavUIProps> = (props) => {
  const { state } = useAccount();
  const { t } = useTranslation();

  const { primaryMenus, secondaryMenus } = useMemo(() => {
    const primary = (props?.menus || []).filter((m) => !m.isSecondary);
    const secondary = (props?.menus || []).filter((m) => m.isSecondary);
    return { primaryMenus: primary, secondaryMenus: secondary };
  }, [props.menus]);

  const showSubAccount = useMemo(
    () => state.status >= AccountStatusEnum.EnableTrading,
    [state.status],
  );

  const onRouteChange = (option: RouteOption) => {
    props.routerAdapter?.onRouteChange?.(option);
    props.hideSheet?.();
  };

  const subAccountTrigger = useMemo(() => {
    const name =
      state.accountId === state.mainAccountId
        ? state.address
        : state.subAccounts?.find((item) => item.id === state.accountId)
            ?.description || "";
    return (
      <div className="oui-shell-action oui-flex oui-w-full oui-items-center oui-gap-2 oui-rounded-xl oui-bg-base-5 oui-p-3">
        <div>
          <PeopleIcon />
        </div>
        <div className="oui-flex oui-flex-col oui-gap-1 oui-font-semibold">
          <Text.formatted
            rule="address"
            className="oui-text-sm oui-text-base-contrast"
          >
            {name}
          </Text.formatted>
          <Text.formatted
            rule="address"
            className="oui-text-2xs oui-text-base-contrast-80"
          >{`ID: ${state.accountId}`}</Text.formatted>
        </div>
        <div className="oui-ms-auto">
          <SwapHorizIcon />
        </div>
      </div>
    );
  }, [state, t]);

  return (
    <SimpleDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title="Navigation"
      classNames={{
        content:
          "oui-platform-navigation oui-shell-overlay oui-w-72 oui-bg-base-8",
      }}
    >
      <div className="oui-relative oui-flex oui-h-full oui-flex-col oui-gap-3">
        <div className="oui-mt-2 oui-flex oui-h-11 oui-items-center [&_img]:!oui-h-5">
          <MainLogo {...props?.logo} />
        </div>
        {props?.leading}
        {showSubAccount && (
          <SubAccountWidget customTrigger={subAccountTrigger} />
        )}
        {(primaryMenus.length > 0 || secondaryMenus.length > 0) && (
          <div className="oui-flex oui-min-h-0 oui-flex-1 oui-flex-col oui-items-start oui-overflow-y-auto">
            {primaryMenus.map((item) => (
              <NavItem
                item={item}
                key={`item-${item.name}`}
                onClick={onRouteChange}
              />
            ))}
            {secondaryMenus.length > 0 && primaryMenus.length > 0 && (
              <Divider className="oui-my-1 oui-w-full" intensity={8} />
            )}
            {secondaryMenus.map((item) => (
              <NavItem
                item={item}
                key={`item-${item.name}`}
                onClick={onRouteChange}
              />
            ))}
          </div>
        )}
        <div className="oui-shrink-0 oui-py-4 oui-flex oui-w-full oui-flex-col oui-gap-4 oui-bg-base-8">
          <div className="oui-flex oui-items-center oui-justify-around">
            {props.telegramUrl && (
              <a
                href={props.telegramUrl}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Telegram"
                className="oui-shell-icon-button"
              >
                <CommunityTG width={24} height={24} />
              </a>
            )}
            {props.twitterUrl && (
              <a
                href={props.twitterUrl}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="X"
                className="oui-shell-icon-button"
              >
                <CommunityX width={24} height={24} />
              </a>
            )}
            {props.discordUrl && (
              <a
                href={props.discordUrl}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Discord"
                className="oui-shell-icon-button"
              >
                <CommunityDiscord width={24} height={24} />
              </a>
            )}
            {props.duneUrl && (
              <a
                href={props.duneUrl}
                target="_blank"
                rel="noopener noreferrer"
                aria-label="Dune"
                className="oui-shell-icon-button"
              >
                <CommunityDune />
              </a>
            )}
          </div>
          {props.customFooter ?? (
            <DefaultTradingViewTncLink className="oui-text-center" />
          )}
        </div>
      </div>
    </SimpleDialog>
  );
};

type NavItemProps = {
  item: LeftNavItem;
  onClick?: (option: RouteOption) => void;
};

const NavItem: FC<NavItemProps> = ({ item, onClick }) => {
  const {
    href,
    name,
    icon,
    trailing,
    onlyInMainAccount,
    customRender,
    target,
  } = item;
  const { isMainAccount } = useAccount();
  const onItemClick = (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      target ||
      !onClick ||
      href.startsWith("http")
    )
      return;
    event.preventDefault();
    // Check if href is a full URL (external link)
    const isExternalLink =
      href.startsWith("http://") || href.startsWith("https://");

    if (target) {
      window.open(href, target);
    } else if (isExternalLink) {
      window.location.href = href;
    } else {
      onClick?.({ href: href, name: name, scope: "leftNav" });
    }
  };
  if (typeof customRender === "function") {
    return (
      <a
        href={href}
        target={target}
        rel={target === "_blank" ? "noopener noreferrer" : undefined}
        className="oui-flex oui-min-h-12 oui-w-full oui-items-center oui-p-3"
        onClick={onItemClick}
      >
        {customRender({ name: name, href: href })}
      </a>
    );
  }
  if (onlyInMainAccount && !isMainAccount) {
    return null;
  }
  const isSecondary = !!item.isSecondary;
  const textClassName = isSecondary
    ? "oui-text-xs oui-font-normal oui-text-base-contrast-80"
    : "oui-text-base oui-font-semibold oui-text-base-contrast-80";
  return (
    <a
      href={href}
      target={target}
      rel={target === "_blank" ? "noopener noreferrer" : undefined}
      className={`oui-flex oui-min-h-12 oui-w-full oui-items-center oui-gap-2 oui-p-3 ${isSecondary ? "oui-py-2" : ""}`}
      onClick={onItemClick}
    >
      <div>{icon}</div>
      <div className={textClassName}>{name}</div>
      {trailing}
    </a>
  );
};
