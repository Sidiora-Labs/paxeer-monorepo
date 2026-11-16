import { FC, ReactNode } from "react";
import { Flex } from "@paxeer-network/ui";
import { ArrowDownIcon } from "../../icons";

type ExchangeDividerProps = {
  icon?: ReactNode;
};

export const ExchangeDivider: FC<ExchangeDividerProps> = ({ icon }) => {
  return (
    <Flex height="var(--oui-space-10)" gapX={3}>
      <Flex
        height="var(--oui-space-0)"
        className="oui-flex-1 oui-bg-base-contrast-12"
      ></Flex>
      {icon || <ArrowDownIcon className="oui-text-primary-light" />}
      <Flex
        height="var(--oui-space-0)"
        className="oui-flex-1 oui-bg-base-contrast-12"
      ></Flex>
    </Flex>
  );
};
