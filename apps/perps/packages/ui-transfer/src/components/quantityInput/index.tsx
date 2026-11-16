import styles from "../../presentation.module.css";
import { FC, ReactNode, useId, useMemo, useRef, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { API } from "@paxeer-network/types";
import {
  Input,
  Select,
  InputProps,
  cn,
  Box,
  Text,
  Flex,
  inputFormatter,
  Spinner,
  InputFormatter,
  useDocumentDirection,
} from "@paxeer-network/ui";
import { Decimal } from "@paxeer-network/utils";
import { InputStatus } from "../../types";
import { TokenOption } from "./tokenOption";

export type QuantityInputProps = {
  token?: API.TokenInfo;
  tokens?: API.TokenInfo[];
  label?: string;
  status?: InputStatus;
  hintMessage?: string;
  hintSuffix?: ReactNode;
  onValueChange?: (value: string) => void;
  onTokenChange?: (token: any) => void;
  loading?: boolean;
  testId?: string;
  formatters?: InputFormatter[];
  // TOOD: remove this prop
  vaultBalanceList?: API.VaultBalance[];
  displayType?: "balance" | "vaultBalance";
  tokenValueFormatter?: (value: string) => ReactNode;
  tokenShowCaret?: boolean;
  balancesRevalidating?: boolean;
  showBalance?: boolean;
  /**
   * Token label display mode.
   * - `false` (default): platform-side paxeer-network account assets — symbol only
   * - `true`: wallet-side chain assets — `display_name || symbol`
   */
  showDisplayName?: boolean;
} & Omit<InputProps, "onClear" | "suffix" | "onValueChange">;

export const QuantityInput: FC<QuantityInputProps> = (props) => {
  const {
    token,
    tokens = [],
    classNames,
    label,
    status,
    hintMessage,
    hintSuffix,
    value,
    onValueChange,
    onTokenChange,
    loading,
    placeholder,
    formatters,
    vaultBalanceList,
    displayType,
    tokenValueFormatter,
    tokenShowCaret,
    balancesRevalidating,
    showBalance,
    showDisplayName = false,
    ...rest
  } = props;

  const { t } = useTranslation();
  const dir = useDocumentDirection();

  const inputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const generatedId = useId();
  const inputId = props.id ?? generatedId;
  const messageId = `${inputId}-hint`;
  const [interacted, setInteracted] = useState(false);
  const [editing, setEditing] = useState(false);
  const displayStatus =
    status === "error" && (!interacted || editing) ? undefined : status;
  const displayHint =
    status === "error" && !displayStatus ? undefined : hintMessage;

  const getTokenLabel = (tokenInfo: API.TokenInfo) =>
    showDisplayName
      ? tokenInfo.display_name || tokenInfo.symbol!
      : tokenInfo.symbol!;

  const tokenOptions = useMemo(() => {
    return tokens.map((token) => {
      const currentToken = vaultBalanceList?.find(
        (item) => item.token === token.symbol,
      );
      const insufficientBalance = currentToken
        ? new Decimal(currentToken.balance ?? 0).lt(value ? Number(value) : 0)
        : false;
      return {
        ...token,
        name: getTokenLabel(token),
        insufficientBalance,
      };
    });
  }, [tokens, value, vaultBalanceList, showDisplayName]);

  const _onTokenChange = (value: string) => {
    const find =
      tokens.find((item) => getTokenLabel(item) === value) ??
      tokens.find((item) => item.symbol === value);
    if (find) {
      onTokenChange?.(find);
    }
  };

  const selectable = tokens.length > 1;
  const selectOpen = selectable ? open : false;

  const optionRenderer = (item: any) => {
    const isActive = item.symbol === token?.symbol;
    return (
      <TokenOption
        token={item}
        isActive={isActive}
        displayType={displayType}
        onTokenChange={(item) => {
          onTokenChange?.(item);
          setOpen(false);
        }}
        open={selectOpen}
        isLoading={balancesRevalidating}
        showBalance={showBalance}
      />
    );
  };

  const prefix = (
    <label htmlFor={inputId} className={styles.quantityLabel}>
      {label || t("common.quantity")}
      {loading && <Spinner size="sm" />}
    </label>
  );

  const tokenLabel = token ? getTokenLabel(token) : undefined;

  const suffix = (
    <div className={styles.quantityToken}>
      <Select.tokens
        open={selectOpen}
        onOpenChange={setOpen}
        disabled={rest.disabled}
        variant="text"
        tokens={tokenOptions}
        value={tokenLabel}
        size={rest.size}
        onValueChange={_onTokenChange}
        showIcon
        valueFormatter={tokenValueFormatter}
        showCaret={tokenShowCaret}
        optionRenderer={optionRenderer}
        contentProps={{
          align: dir === "rtl" ? "start" : "end",
          sideOffset: 5,
          className: cn(
            styles.popover,
            "oui-w-[var(--oui-spacing-sm)] oui-border oui-border-line-6",
          ),
        }}
      />
    </div>
  );

  const message = (
    <Flex
      id={messageId}
      className={styles.hint}
      mt={1}
      gapX={1}
      px={1}
      justify="between"
      itemAlign="center"
    >
      <Flex gapX={1} itemAlign="center">
        <Box
          width="var(--oui-space-1)"
          height="var(--oui-space-1)"
          r="full"
          className={cn(
            displayStatus === "error" && "oui-bg-danger-light",
            displayStatus === "warning" && "oui-bg-warning-light",
          )}
        ></Box>
        <Text
          size="2xs"
          className={cn(
            displayStatus === "error" && "oui-text-danger-light",
            displayStatus === "warning" && "oui-text-warning-light",
          )}
        >
          {displayHint}
        </Text>
      </Flex>
      {hintSuffix && (
        <Box className="oui-flex oui-items-center oui-gap-1">{hintSuffix}</Box>
      )}
    </Flex>
  );

  const _placeholder = placeholder ?? (loading ? "" : "0");

  return (
    <>
      <Input
        ref={inputRef}
        autoComplete="off"
        placeholder={_placeholder}
        prefix={prefix}
        suffix={suffix}
        value={value}
        onValueChange={(value) => {
          onValueChange?.(value);
        }}
        formatters={
          formatters || [
            inputFormatter.numberFormatter,
            inputFormatter.dpFormatter(token?.precision ?? 2),
            inputFormatter.currencyFormatter,
          ]
        }
        {...rest}
        id={inputId}
        name={props.name ?? "quantity"}
        inputMode="decimal"
        aria-invalid={displayStatus === "error"}
        aria-describedby={
          displayHint
            ? [props["aria-describedby"], messageId].filter(Boolean).join(" ")
            : props["aria-describedby"]
        }
        aria-busy={loading}
        onFocus={(event) => {
          setEditing(true);
          props.onFocus?.(event);
        }}
        onBlur={(event) => {
          setEditing(false);
          setInteracted(true);
          props.onBlur?.(event);
        }}
        classNames={{
          ...classNames,
          root: cn(
            styles.quantity,
            "oui-rounded-lg",
            displayStatus === "error" &&
              "oui-outline-danger-light focus-within:oui-outline-danger-light",
            displayStatus === "warning" &&
              "oui-outline-warning-light focus-within:oui-outline-warning-light",
            props.readOnly && "focus-within:oui-outline-0",
            classNames?.root,
          ),
          input: cn("oui-min-w-0 oui-text-base", classNames?.input),
        }}
      />
      {displayHint && message}
    </>
  );
};
