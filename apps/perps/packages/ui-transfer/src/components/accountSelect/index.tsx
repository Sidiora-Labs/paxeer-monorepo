import { useState } from "react";
import { SubAccount } from "@paxeer-network/hooks";
import {
  Box,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  Flex,
  ScrollArea,
  Text,
  cn,
} from "@paxeer-network/ui";
import { ExchangeIcon } from "../../icons";

type AccountSelectProps = {
  subAccounts?: SubAccount[];
  value?: SubAccount;
  onValueChange?: (subAccount: SubAccount) => void;
};

export const AccountSelect: React.FC<AccountSelectProps> = (props) => {
  const { subAccounts = [], value } = props;
  const [open, setOpen] = useState(false);

  const selectable = subAccounts.length > 1;

  const trigger = (
    <Flex
      asChild
      className={cn(
        "oui-rounded-b-sm oui-rounded-t-xl oui-bg-base-6",
        selectable ? "oui-cursor-pointer" : "oui-cursor-auto",
      )}
      height="var(--oui-space-12)"
      px={3}
      justify="between"
      itemAlign="center"
    >
      <button
        type="button"
        disabled={!selectable}
        className="oui-w-full oui-text-start oui-min-h-12"
      >
        <Flex direction="column" itemAlign="start">
          <Text.formatted
            size="xs"
            intensity={80}
            rule={value?.description ? undefined : "address"}
          >
            {value?.description || value?.id}
          </Text.formatted>
          <Text intensity={80} size="2xs">
            {`ID: `}
            <Text.formatted rule="address">{value?.id}</Text.formatted>
          </Text>
        </Flex>
        {selectable && <ExchangeIcon className="oui-text-base-contrast-80" />}
      </button>
    </Flex>
  );

  const content = subAccounts.map((subAccount, index) => {
    const isActive = subAccount.id === value?.id;

    const asset = subAccount?.holding?.find(
      (item) => item.token === "USDC",
    )?.holding;

    return (
      <DropdownMenuItem asChild key={subAccount.id}>
        <Flex
          asChild
          key={subAccount.id}
          px={2}
          r="base"
          justify="between"
          className={cn(
            "oui-transfer-form-account-select-item",
            "oui-cursor-pointer oui-py-1.5 hover:oui-bg-base-5",
            isActive && "oui-bg-base-5",
            index !== 0 && "oui-mt-0.5",
          )}
        >
          <button
            type="button"
            onClick={async () => {
              setOpen(false);
              props.onValueChange?.(subAccount);
            }}
            className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-items-center oui-justify-center oui-rounded hover:oui-bg-base-contrast-4 focus-visible:oui-outline focus-visible:oui-outline-2 focus-visible:oui-outline-primary disabled:oui-cursor-not-allowed"
          >
            <Flex gapX={1} itemAlign="center">
              <Flex
                direction="column"
                itemAlign="start"
                className="oui-text-2xs"
              >
                <Text.formatted
                  rule={subAccount?.description ? undefined : "address"}
                  intensity={80}
                >
                  {subAccount?.description || subAccount?.id}
                </Text.formatted>
                <Text intensity={80} size="2xs">
                  {`ID: `}
                  <Text.formatted rule="address">
                    {subAccount?.id}
                  </Text.formatted>
                </Text>
              </Flex>
            </Flex>
            <Flex gapX={3}>
              <Text.numeral size="2xs" intensity={80} unit=" USDC">
                {asset || 0}
              </Text.numeral>
              {isActive && (
                <Box
                  width="var(--oui-space-1)"
                  height="var(--oui-space-1)"
                  r="full"
                  className="oui-transfer-form-account-select-active-dot oui-gradient-brand"
                />
              )}
            </Flex>
          </button>
        </Flex>
      </DropdownMenuItem>
    );
  });

  return (
    <DropdownMenuRoot open={selectable ? open : false} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>{trigger}</DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent
          align="start"
          sideOffset={4}
          className={cn(
            "oui-transfer-form-account-select-dropdown-menu-content",
            "oui-font-semibold",
            "oui-bg-base-8 oui-p-1",
            "oui-w-[var(--radix-dropdown-menu-trigger-width)] oui-max-w-[calc(100vw-var(--oui-space-6))] [&_[role=menuitem]]:oui-min-h-12 [&_[role=menuitem]]:oui-h-auto",
            "oui-select-none oui-rounded-md",
          )}
        >
          <ScrollArea>
            <div className="oui-max-h-64">{content} </div>
          </ScrollArea>
        </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenuRoot>
  );
};
