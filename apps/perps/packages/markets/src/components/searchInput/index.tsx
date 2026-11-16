import styles from "../../presentation.module.css";
import React, { useRef, useCallback } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  CloseCircleFillIcon,
  Input,
  InputProps,
  cn,
} from "@paxeer-network/ui";
import { SearchIcon } from "../../icons";
import { useMarketsContext } from "../marketsProvider";

type SearchInputProps = Pick<InputProps, "classNames" | "suffix">;

export const SearchInput: React.FC<SearchInputProps> = (props) => {
  const { classNames, suffix } = props;
  const isComposingRef = useRef<boolean>(false);

  const { t } = useTranslation();

  const { searchValue, onSearchValueChange, clearSearchValue } =
    useMarketsContext();

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
  }, []);

  const handleCompositionEnd = useCallback(
    (event: React.CompositionEvent<HTMLInputElement>) => {
      isComposingRef.current = false;
      onSearchValueChange?.(event.currentTarget.value);
    },
    [onSearchValueChange],
  );

  return (
    <Input
      type="search"
      name="market-search"
      aria-label={t("markets.search.placeholder")}
      autoCapitalize="none"
      spellCheck={false}
      value={searchValue}
      onValueChange={onSearchValueChange}
      onCompositionStart={handleCompositionStart}
      onCompositionEnd={handleCompositionEnd}
      placeholder={t("markets.search.placeholder")}
      classNames={{ ...classNames, root: cn(styles.search, classNames?.root) }}
      size="sm"
      prefix={
        <Box pl={3} pr={1}>
          <SearchIcon className="oui-text-base-contrast-80" />
        </Box>
      }
      suffix={
        suffix ||
        (searchValue && (
          <Box mr={2}>
            <button
              type="button"
              aria-label={t("common.clear")}
              onClick={clearSearchValue}
              className="oui-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center"
            >
              <CloseCircleFillIcon
                size={14}
                className="oui-cursor-pointer oui-text-base-contrast-80"
              />
            </button>
          </Box>
        ))
      }
      autoComplete="off"
    />
  );
};
