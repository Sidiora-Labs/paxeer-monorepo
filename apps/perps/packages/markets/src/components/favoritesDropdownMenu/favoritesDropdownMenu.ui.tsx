import styles from "../../presentation.module.css";
import { PropsWithChildren } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  Button,
  CheckedSquareFillIcon,
  CheckSquareEmptyIcon,
  PlusIcon,
  CloseIcon,
  Divider,
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
  Flex,
  Input,
  Text,
  cn,
  CloseCircleFillIcon,
  Tooltip,
} from "@paxeer-network/ui";
import { SymbolBadge } from "../symbolBadge";
import type { UseFavoritesDropdownMenuScriptReturn } from "./favoritesDropdownMenu.script";

export type FavoritesDropdownMenuProps =
  PropsWithChildren<UseFavoritesDropdownMenuScriptReturn>;

export const FavoritesDropdownMenu: React.FC<FavoritesDropdownMenuProps> = (
  props,
) => {
  const {
    symbol,
    favoriteTabs,
    open,
    onOpenChange,
    inputVisible,
    selectedTabs,
    value,
    onValueChange,
    hide,
    hideInput,
    showInput,
    onCheck,
    addTab,
    confirm,
  } = props;

  const { t } = useTranslation();

  const overLen = value?.length > 15;

  const renderInput = () => {
    if (inputVisible) {
      return (
        <Box>
          <Flex gapX={2}>
            <Input
              autoFocus
              name="watchlist-name"
              aria-label={t("markets.favorites.dropdown.addPlaceholder")}
              aria-invalid={overLen}
              value={value}
              onValueChange={onValueChange}
              classNames={{
                root: cn(
                  "oui-ms-px oui-h-7 oui-w-full oui-rounded-sm oui-bg-base-6",
                  overLen &&
                    "focus-within:oui-outline-danger focus-visible:oui-outline-danger",
                ),
              }}
              autoComplete="off"
              suffix={
                value && (
                  <Box mr={2}>
                    <button
                      type="button"
                      onClick={() => {
                        onValueChange("");
                      }}
                      aria-label="Close"
                      className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                    >
                      <CloseCircleFillIcon
                        opacity={1}
                        size={14}
                        className="oui-cursor-pointer oui-text-base-contrast-80"
                      />
                    </button>
                  </Box>
                )
              }
            />

            <Button
              className="oui-rounded-sm"
              size="sm"
              onClick={addTab}
              disabled={!value || overLen}
            >
              {t("common.add")}
            </Button>
          </Flex>

          {overLen && (
            <Flex itemAlign="center" gapX={1} mt={1}>
              <div className="oui-size-1 oui-rounded-full oui-bg-danger"></div>
              <Text color="danger">{t("markets.favorites.tabs.maxName")}</Text>
            </Flex>
          )}
        </Box>
      );
    }

    const overTabs = favoriteTabs.length >= 10;

    return (
      <Tooltip
        open={overTabs ? undefined : false}
        content={
          <Text size="2xs" intensity={80}>
            {t("markets.favorites.tabs.maxList")}
          </Text>
        }
        className="oui-bg-base-6"
        delayDuration={0}
        arrow={{ className: "oui-fill-base-6" }}
      >
        <div>
          <Flex
            asChild
            className={cn(
              overTabs ? "oui-cursor-not-allowed" : "oui-cursor-pointer",
            )}
            itemAlign="center"
            gapX={2}
            p={2}
            intensity={overTabs ? 500 : 600}
            height="var(--oui-space-6)"
            r="base"
          >
            <button
              type="button"
              onClick={overTabs ? undefined : showInput}
              className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
            >
              <PlusIcon
                size={14}
                className="oui-text-base-contrast-80"
                opacity={1}
              />
              <Text className="" intensity={80}>
                {t("markets.favorites.dropdown.addPlaceholder")}
              </Text>
            </button>
          </Flex>
        </div>
      </Tooltip>
    );
  };

  const header = (
    <Flex justify="between" className="oui-mb-2.5 oui-mt-3">
      <Flex gapX={1}>
        {t("markets.favorites.dropdown.title")}
        <Text.formatted
          rule="symbol"
          formatString="base"
          size="base"
          showIcon
          suffix={<SymbolBadge symbol={symbol} />}
        >
          {symbol}
        </Text.formatted>
      </Flex>
      <button
        type="button"
        onClick={hide}
        aria-label="Close"
        className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
      >
        <CloseIcon
          size={16}
          className="oui-cursor-pointer oui-text-base-contrast"
        />
      </button>
    </Flex>
  );

  const content = (
    // <ScrollArea className="custom-scrollbar">
    <Box>
      <Box
        my={2}
        className="oui-custom-scrollbar oui-max-h-72 oui-overflow-y-auto"
      >
        {favoriteTabs?.slice(0, 10)?.map((item) => {
          const checked = !!selectedTabs.find((tab) => tab.id === item.id);
          return (
            <Box key={item.id} className="oui-cursor-pointer">
              <Flex
                asChild
                className="oui-gap-x-1.5 hover:oui-bg-base-6"
                p={2}
                r="md"
              >
                <button
                  type="button"
                  aria-pressed={checked}
                  onClick={() => {
                    onCheck(item, checked);
                  }}
                  className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
                >
                  {checked ? (
                    <CheckedSquareFillIcon
                      size={18}
                      className="oui-text-base-contrast-80"
                    />
                  ) : (
                    <CheckSquareEmptyIcon
                      size={18}
                      className="oui-text-base-contrast-80"
                    />
                  )}

                  <Text intensity={80}>{item.name}</Text>
                </button>
              </Flex>
            </Box>
          );
        })}
      </Box>
      <Box mt={3} pb={5}>
        {renderInput()}
      </Box>
    </Box>
    // </ScrollArea>
  );

  const footer = (
    <Flex gapX={3} mt={3}>
      <Button
        key="secondary"
        color="gray"
        onClick={hide}
        fullWidth
        className="oui-text-sm"
        size="md"
      >
        {t("common.cancel")}
      </Button>

      <Button
        key="primary"
        onClick={confirm}
        fullWidth
        className="oui-text-sm"
        size="md"
      >
        {t("common.confirm")}
      </Button>
    </Flex>
  );

  return (
    <PopoverRoot open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={t("markets.favorites")}
          onClick={(event) => event.stopPropagation()}
          className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-shrink-0 oui-items-center oui-justify-center"
        >
          {props.children}
        </button>
      </PopoverTrigger>
      <PopoverContent
        onClick={(e) => e.stopPropagation()}
        align="start"
        sideOffset={4}
        className={cn(
          styles.surface,
          "oui-markets-favorite-dropdown-menu-content oui-bg-base-8 oui-w-96 oui-max-w-[calc(100vw-var(--oui-space-6))] oui-max-h-[var(--radix-popover-content-available-height)] oui-overflow-auto",
        )}
      >
        <Box px={5} pb={5} width="100%">
          <Text as="div" size="sm" weight="semibold">
            {header}
            <Divider />
            {content}
            {footer}
          </Text>
        </Box>
      </PopoverContent>
    </PopoverRoot>
  );
};
