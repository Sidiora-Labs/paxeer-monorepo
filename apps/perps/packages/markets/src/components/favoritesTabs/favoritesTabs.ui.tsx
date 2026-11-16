import styles from "../../presentation.module.css";
import { KeyboardEventHandler } from "react";
import { FavoriteTab } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  cn,
  Flex,
  Text,
  Tooltip,
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
  Input,
  modal,
} from "@paxeer-network/ui";
import { AddIcon, ActiveAddIcon, EditIcon, TrashIcon } from "../../icons";
import {
  UseFavoritesTabScriptOptions,
  UseFavoritesTabScriptReturn,
} from "./favoritesTabs.script";

export type FavoritesTabProps = UseFavoritesTabScriptReturn &
  Pick<UseFavoritesTabScriptOptions, "size"> & {
    className?: string;
  };

export const FavoritesTab: React.FC<FavoritesTabProps> = (props) => {
  const {
    open,
    setOpen,
    container,
    inputRef,
    editing,
    value,
    onValueChange,
    updateCurTab,
    onEdit,
    addTab,
    delTab,
    size = "default",
    scrollable,
  } = props;

  const { t } = useTranslation();

  const { selectedFavoriteTab, favoriteTabs, updateSelectedFavoriteTab } =
    props.favorite;

  const addIconWidth = "var(--oui-space-12)";
  const tabHeight = "var(--oui-space-12)";
  const overLen = value?.length > 15;

  const gradientColor =
    "oui-bg-gradient-to-r oui-from-[rgb(var(--oui-gradient-brand-start)_/_0.12)] oui-to-[rgb(var(--oui-gradient-brand-end)_/_0.12)]";

  const onDel = (item: any) => {
    modal.confirm({
      title: t("markets.favorites.tabs.delete.dialog.title"),
      content: (
        <Text size="sm">
          {t("markets.favorites.tabs.delete.dialog.description", {
            name: item.name,
          })}
        </Text>
      ),
      onOk() {
        delTab(item);
        return Promise.resolve();
      },
    });
  };

  const onKeyUp: KeyboardEventHandler = (e) => {
    if (e.key === "Enter") {
      updateCurTab(overLen);
    }
  };

  const renderActions = (item: any) => {
    return (
      <Flex gapX={2} itemAlign="center" px={2} py={1}>
        <button
          type="button"
          onClick={() => {
            onEdit(item);
          }}
          aria-label="Edit"
          className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
        >
          <EditIcon className="oui-cursor-pointer oui-text-base-contrast-80 hover:oui-text-base-contrast" />
        </button>
        <button
          type="button"
          aria-label={t("common.delete")}
          onClick={() => onDel(item)}
          className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-12"
        >
          <TrashIcon className="oui-text-base-contrast-80" />
        </button>
      </Flex>
    );
  };

  const renderAdd = () => {
    const overTabs = favoriteTabs.length >= 10;

    const icon = (
      <Flex
        asChild
        className={cn(
          "oui-inline-flex",
          overTabs ? "oui-cursor-not-allowed" : "oui-cursor-pointer",
          overTabs ? "oui-bg-base-3" : gradientColor,
        )}
        width={addIconWidth}
        height={tabHeight}
        r="base"
        justify="center"
        itemAlign="center"
      >
        <button
          type="button"
          aria-label={t("markets.favorites") + " +"}
          aria-disabled={overTabs}
          onClick={overTabs ? undefined : addTab}
          className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
        >
          {overTabs ? (
            <AddIcon className="oui-z-10 oui-size-3 oui-text-base-contrast-80" />
          ) : (
            <ActiveAddIcon className="oui-size-3" />
          )}
        </button>
      </Flex>
    );

    if (overTabs) {
      return (
        <Tooltip
          // @ts-ignore
          content={
            <Text size="2xs" intensity={80}>
              Maximum 10 groups in the favorite list
            </Text>
          }
          className="oui-bg-base-6"
          delayDuration={0}
          arrow={{ className: "oui-fill-base-6" }}
        >
          {icon}
        </Tooltip>
      );
    }

    return icon;
  };

  const renderInput = (isActive: boolean) => {
    return (
      <Tooltip
        open={overLen}
        // @ts-ignore
        content={
          <Text size="2xs" intensity={80}>
            List name cannot exceed 15 characters
          </Text>
        }
        className="oui-bg-base-6"
        delayDuration={0}
        sideOffset={0}
      >
        <Input
          ref={inputRef}
          aria-label={t("markets.favorites")}
          name="watchlist-name"
          aria-invalid={overLen}
          style={
            overLen
              ? undefined
              : {
                  // @ts-ignore
                  "--oui-gradient-angle": "270deg",
                  backgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                  WebkitBackgroundClip: "text",
                }
          }
          classNames={{
            root: cn(
              styles.watchlistInput,
              "oui-my-px oui-rounded oui-p-0 oui-px-2",
              "focus-within:oui-outline-transparent focus-visible:oui-outline-none",
              size === "sm" ? "oui-h-4" : "oui-h-6",
              isActive && cn(overLen ? "oui-bg-danger/15" : gradientColor),
            ),
            input: cn(
              "oui-font-semibold oui-caret-[rgb(var(--oui-color-tertiary))]",
              overLen
                ? "oui-text-danger"
                : "oui-text-transparent oui-gradient-brand ",
              size === "sm" ? "oui-text-2xs" : "oui-text-sm",
            ),
          }}
          value={value}
          onValueChange={onValueChange}
          onBlur={() => {
            updateCurTab(overLen);
          }}
          onKeyUp={onKeyUp}
          autoComplete="off"
        />
      </Tooltip>
    );
  };

  const renderContent = (item: FavoriteTab, isActive: boolean) => {
    if (editing && isActive) {
      return renderInput(isActive);
    }

    const textProps = {
      weight: "semibold",
      size: size === "sm" ? "2xs" : "sm",
      className: size === "sm" ? "oui-leading-normal" : "oui-leading-normal",
      as: "div",
    } as any;

    const content = isActive ? (
      <Text.gradient color="brand" angle={270} {...textProps}>
        {item.name}
      </Text.gradient>
    ) : (
      <Text {...textProps}>{item.name}</Text>
    );

    return (
      <Box
        asChild
        r="base"
        px={2}
        height={tabHeight}
        className={cn(
          "oui-cursor-pointer oui-select-none",
          isActive
            ? "oui-markets-favorites-active-tab-item"
            : "oui-markets-favorites-tab-item",
          isActive
            ? gradientColor
            : "oui-bg-line-6 oui-text-base-contrast-80 hover:oui-text-base-contrast",
        )}
      >
        <button
          type="button"
          onClick={() => {
            updateSelectedFavoriteTab(item);
            if (isActive) {
              setOpen(true);
            }
          }}
          className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
        >
          {content}
        </button>
      </Box>
    );
  };

  return (
    <Flex width="100%" gapX={3} className={cn(styles.surface, props.className)}>
      <Flex
        ref={container}
        id="oui-markets-favorites-tabs-container"
        className={cn(
          "oui-hide-scrollbar oui-relative oui-cursor-pointer",
          "oui-overflow-hidden oui-overflow-x-auto",
        )}
        // my={3}
        gapX={size === "sm" ? 2 : 3}
        width="100%"
      >
        {favoriteTabs?.slice(0, 10)?.map((item: any) => {
          const isActive = selectedFavoriteTab.id === item.id;
          return (
            <PopoverRoot
              key={item.id}
              open={isActive && !editing ? open : false}
              onOpenChange={(open) => {
                if (isActive) setOpen(open);
              }}
            >
              {editing && isActive ? (
                renderInput(isActive)
              ) : (
                <PopoverTrigger asChild>
                  {renderContent(item, isActive)}
                </PopoverTrigger>
              )}
              <PopoverContent
                aria-label={item.name}
                className={cn(
                  styles.surface,
                  "oui-w-auto oui-max-w-[calc(100vw-var(--oui-space-6))] oui-bg-base-5 oui-p-1",
                )}
              >
                {renderActions(item)}
              </PopoverContent>
            </PopoverRoot>
          );
        })}

        {!scrollable && renderAdd()}
      </Flex>
      {scrollable && renderAdd()}
    </Flex>
  );
};
