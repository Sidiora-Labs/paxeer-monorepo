import {
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
} from "../../../../ui-connector/src/platformOverlays";
import React, { useState } from "react";
import {
  Flex,
  Text,
  SpotIcon,
  PerpsIcon,
  SelectedChoicesFillIcon,
  CloseRoundFillIcon,
} from "@paxeer-network/ui";
import { cn } from "@paxeer-network/ui";

type TradingMenuChoiceProps = {
  icon: React.ReactNode;
  label: string;
  selected?: boolean;
  onClick?: () => void;
  className?: string;
};

const TradingMenuChoice: React.FC<TradingMenuChoiceProps> = (props) => {
  const { icon, label, selected, onClick, className } = props;
  return (
    <Flex
      asChild
      itemAlign="center"
      className={cn(
        "oui-w-full oui-px-2 oui-py-3",
        "oui-rounded-md",
        "oui-cursor-pointer",
        className,
      )}
      onClick={onClick}
    >
      <button type="button" className="oui-shell-action">
        <Flex gap={2} itemAlign="center">
          {icon}
          <Text size="sm" intensity={80} weight="semibold">
            {label}
          </Text>
        </Flex>
        <Flex className="oui-ms-auto">
          {selected ? <SelectedChoicesFillIcon size={14} /> : null}
        </Flex>
      </button>
    </Flex>
  );
};

export type BottomNavTradingMenuProps = {
  active: boolean;
  activeIcon: React.ReactNode;
  inactiveIcon: React.ReactNode;
  label: string;
  onNavigate: () => void;
};

export const BottomNavTradingMenu: React.FC<BottomNavTradingMenuProps> = (
  props,
) => {
  const { active, activeIcon, inactiveIcon, onNavigate, label } = props;
  const [open, setOpen] = useState(false);

  const onClickTrigger = () => {
    if (!active) {
      onNavigate();
      return;
    }
    setOpen((v) => !v);
  };

  return (
    <PopoverRoot
      open={open}
      onOpenChange={(next) => {
        if (active) setOpen(next);
      }}
    >
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={open ? "Close trading menu" : label}
          aria-current={active ? "page" : undefined}
          onClick={() => {
            if (!active) onNavigate();
          }}
          className="oui-shell-action oui-flex oui-flex-col oui-items-center oui-justify-center oui-gap-1"
        >
          {open ? (
            <CloseRoundFillIcon size={28} />
          ) : active ? (
            activeIcon
          ) : (
            inactiveIcon
          )}
          <Text size="2xs" intensity={active ? 98 : 80}>
            {label}
          </Text>
        </button>
      </PopoverTrigger>
      <PopoverContent
        sideOffset={12}
        align="center"
        side="top"
        className="!oui-w-48 !oui-bg-base-8 !oui-p-1 oui-border-none -oui-mb-6"
      >
        <Flex direction="column" className="oui-gap-1 oui-w-full">
          <TradingMenuChoice
            icon={<SpotIcon size={20} />}
            label="Spot"
            onClick={() => {
              window.location.href = "https://woofi.com/swap";
              setOpen(false);
            }}
            className=""
          />
          <TradingMenuChoice
            icon={<PerpsIcon size={20} />}
            label="Perps"
            selected={true}
            onClick={() => setOpen(false)}
            className="oui-bg-base-6"
          />
        </Flex>
      </PopoverContent>
    </PopoverRoot>
  );
};
