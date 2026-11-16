import { Tooltip } from "../../../../../ui-connector/src/platformOverlays";
import { useCallback } from "react";
import type { MouseEvent } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  cn,
  CopyIcon,
  Flex,
  formatAddress,
  Text,
  toast,
  useScreen,
} from "@paxeer-network/ui";
import { EditIcon } from "../icons";

interface AccountItemProps {
  isMainAccount?: boolean;
  accountId: string;
  description?: string;
  userAddress?: string;
  balance?: string;
  isCurrent?: boolean;
  onSwitch?: (accountId: string) => void;
  accountValue?: number;
  onEdit?: (accountItem: { accountId: string; description: string }) => void;
}

function splitAccountIdForDisplay(accountId: string) {
  return {
    leading: accountId.slice(0, 6),
    middle: accountId.slice(6, -4),
    trailing: accountId.slice(-4),
  };
}

interface AccountIdForCopyProps {
  accountId: string;
  onCopy: (event: MouseEvent) => void;
}

const AccountIdForCopy = (props: AccountIdForCopyProps) => {
  const info = splitAccountIdForDisplay(props.accountId);
  return (
    <Flex
      className="oui-min-h-12 oui-w-48"
      gap={2}
      justify="between"
      itemAlign="center"
    >
      <Text className="oui-w-full oui-break-all oui-text-2xs oui-text-base-contrast-80">
        <Text className="oui-text-base-contrast">{info.leading}</Text>
        <Text>{info.middle}</Text>
        <Text className="oui-text-base-contrast">{info.trailing}</Text>
      </Text>
      <button
        type="button"
        className="oui-shell-icon-button oui-pointer-events-auto"
        aria-label="Copy"
        onClick={props.onCopy}
      >
        <CopyIcon className="oui-cursor-pointer oui-text-base-contrast-80 hover:oui-text-base-contrast" />
      </button>
    </Flex>
  );
};

export const AccountItem = (props: AccountItemProps) => {
  const { t } = useTranslation();
  const { isMobile } = useScreen();

  const {
    isMainAccount,
    accountId,
    description,
    userAddress,
    isCurrent,
    onSwitch,
    accountValue,
    onEdit,
  } = props;

  const copyAccountId = useCallback(
    (event: MouseEvent) => {
      event.stopPropagation();
      navigator.clipboard.writeText(accountId);
      toast.success(t("common.copy.copied"));
    },
    [accountId, t],
  );

  const handleSwitch = useCallback(() => {
    if (isCurrent) return;
    onSwitch?.(accountId);
  }, [isCurrent, onSwitch, accountId]);

  const handleEdit = useCallback(
    (event: MouseEvent) => {
      event.stopPropagation();
      event.preventDefault();
      onEdit?.({
        accountId,
        description: description ?? "",
      });
    },
    [onEdit, accountId, description],
  );

  const stopPropagation = useCallback((event: MouseEvent) => {
    event.stopPropagation();
  }, []);

  return (
    <Flex
      justify="between"
      itemAlign="center"
      width="100%"
      className={cn(
        "oui-relative oui-cursor-pointer oui-rounded-md oui-bg-base-6 oui-px-3 oui-py-4",
        "oui-border oui-border-base-6",
        isCurrent && "oui-border-[rgb(var(--oui-gradient-brand-start))]",
        !isCurrent && "hover:oui-border-base-contrast-16",
      )}
    >
      <button
        type="button"
        aria-label={`Switch to ${description || userAddress || accountId}`}
        aria-pressed={!!isCurrent}
        className="oui-absolute oui-inset-0 oui-z-0 oui-rounded-md"
        onClick={handleSwitch}
      />
      {isCurrent && (
        <div
          className={cn(
            "oui-absolute -oui-end-px -oui-top-px oui-leading-normal",
            "oui-text-3xs oui-font-semibold oui-text-base-10",
            "oui-rounded-md oui-rounded-ee-none oui-rounded-ss-none oui-bg-[rgb(var(--oui-gradient-brand-start))] oui-py-0.5 oui-ps-1 oui-pe-1",
          )}
        >
          {t("subAccount.modal.current")}
        </div>
      )}
      <Flex
        direction="column"
        itemAlign="start"
        gap={1}
        className="oui-pointer-events-none oui-z-[2]"
      >
        {isMainAccount ? (
          <Text className="oui-text-xs oui-leading-normal oui-text-base-contrast">
            {formatAddress(userAddress ?? "")}
          </Text>
        ) : (
          <Flex
            asChild
            justify="start"
            itemAlign="center"
            className="oui-cursor-pointer oui-gap-0.5 oui-fill-base-contrast-54 hover:oui-fill-base-contrast"
            onClick={handleEdit}
          >
            <button
              type="button"
              className="oui-shell-action oui-pointer-events-auto"
            >
              <Text className="oui-text-xs oui-leading-normal oui-text-base-contrast">
                {description}
              </Text>
              <EditIcon className="oui-fill-base-contrast-54 hover:oui-fill-base-contrast" />
            </button>
          </Flex>
        )}
        {isMobile ? (
          <Flex
            justify="start"
            itemAlign="center"
            gap={1}
            className="oui-w-full"
            onClick={stopPropagation}
          >
            <Text className="oui-text-2xs oui-leading-normal oui-text-base-contrast-80">
              ID: {formatAddress(accountId)}
            </Text>
            <button
              type="button"
              className="oui-shell-icon-button oui-pointer-events-auto"
              aria-label="Copy"
              onClick={copyAccountId}
            >
              <CopyIcon
                size={12}
                className="oui-shrink-0 oui-cursor-pointer oui-text-base-contrast-80 hover:oui-text-base-contrast"
              />
            </button>
          </Flex>
        ) : (
          <Tooltip content={accountId}>
            <button
              type="button"
              onClick={copyAccountId}
              className="oui-shell-action oui-pointer-events-auto oui-text-start oui-text-2xs oui-text-base-contrast-80"
            >
              ID: {formatAddress(accountId)}
            </button>
          </Tooltip>
        )}
      </Flex>
      <Flex
        className="oui-pointer-events-none oui-text-xs oui-text-base-contrast"
        itemAlign="end"
        gap={1}
      >
        <Text.numeral rule="price" dp={2}>
          {accountValue ?? 0}
        </Text.numeral>
        <Text>USDC</Text>
      </Flex>
    </Flex>
  );
};
