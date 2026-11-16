import styles from "../../presentation.module.css";
import { useId, useRef, useState } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Box,
  cn,
  Flex,
  Text,
  PopoverRoot,
  PopoverAnchor,
  PopoverContent,
} from "@paxeer-network/ui";
import { InputStatus } from "../../types";
import type { AccountInfo } from "../withdrawForm/hooks/useWithdrawAccountId";

export type TextAreaInputProps = {
  value: string;
  onChange: (value: string) => void;
  status?: InputStatus;
  hintMessage?: string;
  placeholder?: string;
  className?: string;
  label: string;
  disabled?: boolean;
  enableAccountLookup?: boolean;
  accountInfo?: AccountInfo | null;
  accountDropdownOpen?: boolean;
  setAccountDropdownOpen?: (open: boolean) => void;
};

export const TextAreaInput = (props: TextAreaInputProps) => {
  const { t } = useTranslation();
  const {
    value,
    onChange,
    status,
    hintMessage,
    placeholder,
    label,
    enableAccountLookup,
    accountInfo,
    accountDropdownOpen,
    setAccountDropdownOpen,
  } = props;
  const inputId = useId();
  const hintId = `${inputId}-hint`;
  const [interacted, setInteracted] = useState(false);
  const [editing, setEditing] = useState(false);
  const skipNextFocus = useRef(false);
  const resultId = `${inputId}-result`;
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [selectedAccount, setSelectedAccount] = useState<AccountInfo | null>(
    null,
  );

  const displayStatus: InputStatus | undefined =
    status === "error" && (!interacted || editing) ? undefined : status;
  const displayHint =
    status === "error" && !displayStatus ? undefined : hintMessage;

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const nextValue = e.target.value;
    if (selectedAccount) {
      setSelectedAccount(null);
    }
    onChange?.(nextValue);
  };

  const prefix = (
    <label htmlFor={inputId} className="oui-block oui-px-3 oui-pt-2">
      <Text size="2xs" intensity={80}>
        {label}
      </Text>
    </label>
  );

  const message = (
    <Flex id={hintId} className={styles.hint} mb={1} gapX={1} px={1}>
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
  );

  const textareaNode = (
    <div className="oui-relative">
      <div
        className={cn(
          "oui-relative oui-w-full oui-rounded-lg oui-mb-1",
          "oui-bg-base-6 oui-text-sm oui-text-base-contrast",
          "oui-outline oui-outline-1 oui-outline-offset-0 oui-outline-transparent focus-within:oui-outline-primary-light",
          displayStatus === "error" &&
            "oui-outline-danger-light focus-within:oui-outline-danger-light",
          displayStatus === "warning" &&
            "oui-outline-warning-light focus-within:oui-outline-warning-light",
          "disabled:oui-cursor-not-allowed",
        )}
      >
        {prefix}
        <textarea
          ref={textareaRef}
          id={inputId}
          name="recipient-account"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={displayStatus === "error"}
          aria-describedby={displayHint ? hintId : undefined}
          onBlur={() => {
            setInteracted(true);
            setEditing(false);
          }}
          placeholder={placeholder}
          className={cn(
            styles.textarea,
            "oui-block oui-w-full oui-bg-transparent",
            "oui-px-3 oui-pt-1",
            selectedAccount?.address ? "oui-pb-0" : "oui-pb-2",
            "oui-text-sm oui-text-base-contrast",
            "oui-rounded-lg oui-outline-none",
            "placeholder:oui-text-base-contrast-80",
            props.className,
          )}
          rows={1}
          value={value}
          onChange={handleChange}
          onKeyDown={(event) => {
            if (
              event.key === "ArrowDown" &&
              accountInfo &&
              accountDropdownOpen
            ) {
              event.preventDefault();
              document.getElementById(resultId)?.focus();
            }
          }}
          onFocus={() => {
            setEditing(true);
            if (skipNextFocus.current) {
              skipNextFocus.current = false;
              return;
            }
            if (accountInfo) {
              setAccountDropdownOpen?.(true);
            }
          }}
          disabled={props.disabled}
        />
        {selectedAccount?.address && (
          <div className="oui-flex oui-items-center oui-justify-between oui-px-3 oui-pb-2 oui-pt-1">
            <Text
              size="2xs"
              intensity={80}
              className="oui-truncate oui-leading-normal"
            >
              ({t("common.address")}:{" "}
              <Text.formatted as="span" rule="address" range={[6, 4]}>
                {selectedAccount.address}
              </Text.formatted>
              )
            </Text>
          </div>
        )}
      </div>
    </div>
  );

  if (!enableAccountLookup) {
    return (
      <div className="oui-relative">
        {textareaNode}
        {displayHint && message}
      </div>
    );
  }

  return (
    <PopoverRoot
      open={!!(accountInfo && accountDropdownOpen)}
      onOpenChange={(open) => {
        if (!open) {
          setAccountDropdownOpen?.(false);
        }
      }}
    >
      <PopoverAnchor asChild>{textareaNode}</PopoverAnchor>
      {displayHint && message}
      {accountInfo && accountDropdownOpen && (
        <PopoverContent
          align="start"
          className={cn(
            "oui-bg-base-8",
            "oui-rounded-md oui-shadow-lg",
            styles.popover,
            "oui-w-[var(--radix-popover-trigger-width)] oui-p-1",
          )}
          onOpenAutoFocus={(event) => event.preventDefault()}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            if (
              document.activeElement === document.body ||
              document.activeElement?.id === resultId
            ) {
              skipNextFocus.current = true;
              textareaRef.current?.focus();
            }
          }}
        >
          <AccountResultItem
            id={resultId}
            item={accountInfo}
            disabled={props.disabled}
            selected={
              !!selectedAccount &&
              selectedAccount.accountId === accountInfo.accountId
            }
            onSelect={(account) => {
              onChange?.(account.accountId);
              setSelectedAccount(account);
              setAccountDropdownOpen?.(false);
            }}
          />
        </PopoverContent>
      )}
    </PopoverRoot>
  );
};

type AccountResultItemProps = {
  id: string;
  item: AccountInfo;
  disabled?: boolean;
  selected?: boolean;
  onSelect: (account: AccountInfo) => void;
};

const AccountResultItem = ({
  id,
  item,
  disabled,
  selected,
  onSelect,
}: AccountResultItemProps) => {
  const { t } = useTranslation();
  return (
    <button
      id={id}
      type="button"
      disabled={disabled}
      aria-pressed={!!selected}
      className={cn(
        styles.option,
        "oui-flex oui-w-full oui-items-center oui-justify-between",
        "oui-gap-1.5 oui-rounded oui-px-2 oui-py-1.5",
        "oui-cursor-pointer hover:oui-bg-base-5",
        disabled && "oui-cursor-not-allowed oui-opacity-50",
      )}
      onClick={() => !disabled && onSelect(item)}
    >
      <div className="oui-flex oui-flex-col oui-items-start oui-gap-0.5">
        <Text
          size="sm"
          intensity={80}
          weight="semibold"
          className="oui-tracking-wide"
        >
          {t("common.accountId")}:{" "}
          <Text.formatted
            rule="address"
            range={[6, 4]}
            as="span"
            className="oui-text-primary-light"
          >
            {item.accountId}
          </Text.formatted>
        </Text>
        <Text size="2xs" intensity={80}>
          ({item.address})
        </Text>
      </div>
      <span aria-hidden="true">{selected ? "✓" : ""}</span>
    </button>
  );
};
