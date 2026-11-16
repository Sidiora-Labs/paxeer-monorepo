import {
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
} from "../../../ui-connector/src/platformOverlays";
import React from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { Box, CloseIcon, cn, Divider, Flex, Text } from "@paxeer-network/ui";
import {
  MarketLeftIcon,
  MarketTopIcon,
  MarketBottomIcon,
  MarketHideIcon,
} from "./icons";
import { OrderEntryIcon } from "./icons/OrderEntryIcon";

type LayoutPosition = "left" | "right";

// Local copy of market layout positions
type MarketLayoutPosition = "left" | "top" | "bottom" | "hide";

interface LayoutSwitchButtonProps {
  layout: LayoutPosition;
  markets: MarketLayoutPosition;
  onLayoutChange: (layout: LayoutPosition) => void;
  onMarketChange: (markets: MarketLayoutPosition) => void;
}

/**
 * Dropdown UI for switching between advanced left/right and markets left/top/bottom/hide.
 */
const LayoutSwitchButton: React.FC<LayoutSwitchButtonProps> = ({
  layout,
  markets,
  onLayoutChange,
  onMarketChange,
}) => {
  const { t } = useTranslation();
  const [open, setOpen] = React.useState(false);
  const [hoveredMarket, setHoveredMarket] =
    React.useState<MarketLayoutPosition | null>(null);

  const renderLayoutItem = (position: LayoutPosition) => (
    <Flex
      asChild
      direction="column"
      gapY={2}
      onClick={() => {
        onLayoutChange(position);
        setOpen(false);
      }}
      className="oui-group"
    >
      <button
        type="button"
        aria-pressed={layout === position}
        className="oui-shell-action"
      >
        <Flex
          justify={position === "right" ? "end" : "start"}
          className={cn(
            "oui-h-24 oui-w-full oui-min-w-0",
            "oui-rounded-xl oui-bg-base-10",
            "oui-border-4 oui-border-base-5 group-hover:oui-border-primary-light",
            layout === position && "!oui-border-primary-light",
          )}
        >
          <Box p={1}>
            <OrderEntryIcon />
          </Box>
        </Flex>
        <Text
          size="2xs"
          intensity={80}
          className={cn(
            "oui-text-base-contrast-80 group-hover:oui-text-base-contrast-80",
            layout === position && "oui-text-base-contrast-80",
          )}
        >
          {String(
            position === "right"
              ? t("trading.layout.advanced.right")
              : t("trading.layout.advanced.left"),
          )}
        </Text>
      </button>
    </Flex>
  );

  const renderMarketItem = (position: MarketLayoutPosition) => {
    const isSelected = markets === position;
    const isHovered = hoveredMarket === position;

    const getLabel = () => {
      switch (position) {
        case "left":
          return t("trading.layout.markets.left");
        case "top":
          return t("trading.layout.markets.top");
        case "bottom":
          return t("trading.layout.markets.bottom");
        case "hide":
          return t("trading.layout.markets.hide");
        default:
          return t("trading.layout.markets.left");
      }
    };

    return (
      <Flex
        asChild
        direction="column"
        gapY={2}
        onClick={() => {
          onMarketChange(position);
          setOpen(false);
        }}
        onMouseEnter={() => setHoveredMarket(position)}
        onMouseLeave={() => setHoveredMarket(null)}
        className="oui-group"
      >
        <button
          type="button"
          aria-pressed={isSelected}
          className="oui-shell-action"
        >
          <Flex justify="center" className="oui-h-24 oui-w-full oui-min-w-0">
            {position === "left" && (
              <MarketLeftIcon isSelected={isSelected} isHovered={isHovered} />
            )}
            {position === "top" && (
              <MarketTopIcon isSelected={isSelected} isHovered={isHovered} />
            )}
            {position === "bottom" && (
              <MarketBottomIcon isSelected={isSelected} isHovered={isHovered} />
            )}
            {position === "hide" && (
              <MarketHideIcon isSelected={isSelected} isHovered={isHovered} />
            )}
          </Flex>
          <Text
            size="2xs"
            intensity={80}
            className={cn(
              "oui-text-base-contrast-80 group-hover:oui-text-base-contrast-80",
              isSelected && "oui-text-base-contrast-80",
            )}
          >
            {String(getLabel())}
          </Text>
        </button>
      </Flex>
    );
  };

  const content = (
    <>
      <Flex
        itemAlign="center"
        justify="between"
        mt={3}
        className="oui-mb-3 oui-min-w-0"
      >
        <Text size="base" intensity={98}>
          {t("trading.layout")}
        </Text>
        <button
          type="button"
          className="oui-shell-icon-button"
          aria-label="Close"
          onClick={() => {
            setOpen(false);
          }}
        >
          <CloseIcon
            size={16}
            className="oui-cursor-pointer oui-text-base-contrast-80"
            opacity={0.98}
          />
        </button>
      </Flex>
      <Divider />
      <Flex direction="column" gapY={2} mt={5} itemAlign="start">
        <Text size="xs" intensity={98}>
          {String(t("trading.layout.advanced"))}
        </Text>
        <div className="oui-shell-layout-options">
          {renderLayoutItem("left")}
          {renderLayoutItem("right")}
        </div>
      </Flex>
      <Flex direction="column" gapY={2} mt={5} itemAlign="start">
        <Text size="xs" intensity={98}>
          {String(t("trading.layout.markets"))}
        </Text>
        <div className="oui-shell-layout-options">
          {renderMarketItem("left")}
          {renderMarketItem("top")}
          {renderMarketItem("bottom")}
          {renderMarketItem("hide")}
        </div>
      </Flex>
    </>
  );

  return (
    <PopoverRoot open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Flex
          asChild
          px={3}
          className={cn(
            "oui-rounded-md",
            "oui-min-h-12",
            "oui-cursor-pointer oui-transition-all",
            "oui-bg-base-6 hover:oui-bg-base-4",
            "oui-text-base-contrast-80 hover:oui-text-base-contrast-80",
          )}
          gapX={1}
          ml={3}
          justify="center"
          itemAlign="center"
        >
          <button type="button">
            <Text size="2xs" weight="semibold">
              {t("trading.layout")}
            </Text>
          </button>
        </Flex>
      </PopoverTrigger>
      <PopoverContent
        aria-label={t("trading.layout")}
        onClick={(e) => e.stopPropagation()}
        align="end"
        className={cn(
          "oui-shell-overlay oui-w-[calc(var(--oui-space-24)*6)] oui-bg-base-8 oui-p-5 oui-pt-0 oui-font-semibold",
        )}
      >
        {content}
      </PopoverContent>
    </PopoverRoot>
  );
};

/**
 * Symbol bar trailing control for layout switching.
 * Note: In the fixed layout approach, this component needs to be connected to
 * the layout state management (e.g., via context or props).
 * This is a simplified version that shows the UI but requires external state management.
 */
export const SymbolBarLayoutSwitcher: React.FC<{
  layout?: LayoutPosition;
  marketLayout?: MarketLayoutPosition;
  onLayoutChange?: (layout: LayoutPosition) => void;
  onMarketLayoutChange?: (markets: MarketLayoutPosition) => void;
}> = ({
  layout = "right",
  marketLayout = "left",
  onLayoutChange,
  onMarketLayoutChange,
}) => {
  if (!onLayoutChange || !onMarketLayoutChange) {
    // When no callbacks provided, don't render
    return <></>;
  }

  return (
    <LayoutSwitchButton
      layout={layout}
      markets={marketLayout}
      onLayoutChange={onLayoutChange}
      onMarketChange={onMarketLayoutChange}
    />
  );
};
