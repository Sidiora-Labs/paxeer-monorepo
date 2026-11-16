import React from "react";
import { type RouterAdapter } from "@paxeer-network/types";
import { Text } from "@paxeer-network/ui";
import { BottomNavItem } from "./bottomNav.widget";

export type BottomNavProps = {
  mainMenus?: BottomNavItem[];
  current?: string | string[];
  onRouteChange?: RouterAdapter["onRouteChange"];
};

export const BottomNav: React.FC<BottomNavProps> = ({
  mainMenus,
  current,
  onRouteChange,
}) => {
  if (!mainMenus || mainMenus.every((menu) => menu.href !== current))
    return null;
  return (
    <nav
      aria-label="Primary"
      className="oui-flex oui-min-h-16 oui-w-full oui-items-stretch oui-justify-between oui-border-t oui-border-line-4 oui-bg-base-9 oui-px-3"
    >
      {mainMenus.map((menu) => {
        const active = current === menu.href;
        return (
          <a
            key={menu.name}
            href={menu.href}
            aria-current={active ? "page" : undefined}
            className="oui-flex oui-min-w-0 oui-flex-1 oui-flex-col oui-items-center oui-justify-center oui-gap-1 oui-py-2 oui-text-center"
            onClick={(event) => {
              if (
                onRouteChange &&
                !menu.href.startsWith("http") &&
                event.button === 0 &&
                !event.metaKey &&
                !event.ctrlKey &&
                !event.shiftKey &&
                !event.altKey
              ) {
                event.preventDefault();
                onRouteChange({ href: menu.href, name: menu.name });
              }
            }}
          >
            <Text>{active ? menu.activeIcon : menu.inactiveIcon}</Text>
            <Text size="2xs" intensity={active ? 98 : 80}>
              {menu.name}
            </Text>
          </a>
        );
      })}
    </nav>
  );
};
