import React from "react";
import {
  Flex,
  PopoverRoot,
  PopoverTrigger,
  PopoverContent,
  Text,
} from "@paxeer-network/ui";
import styles from "../../presentation.module.css";

interface DataItemProps {
  label: string;
  value: React.ReactNode;
  hint?: React.ReactNode;
}

export const DataItem: React.FC<DataItemProps> = ({ label, value, hint }) => (
  <Flex direction="column" itemAlign="start">
    {hint ? (
      <PopoverRoot>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="oui-data-label oui-border-b oui-border-dashed oui-border-line-12 oui-text-2xs oui-text-base-contrast-80 oui-whitespace-nowrap"
          >
            {label}
          </button>
        </PopoverTrigger>
        <PopoverContent
          className={`${styles.surface} oui-w-80 oui-max-w-[calc(100vw-var(--oui-space-6))] oui-max-h-[var(--radix-popover-content-available-height)] oui-overflow-auto oui-bg-base-6`}
        >
          {hint}
        </PopoverContent>
      </PopoverRoot>
    ) : (
      <Text
        size="2xs"
        intensity={80}
        className="oui-data-label oui-whitespace-nowrap"
      >
        {label}
      </Text>
    )}
    <Text
      size="2xs"
      intensity={98}
      className="oui-data-value oui-whitespace-nowrap oui-break-normal oui-leading-normal"
    >
      {value}
    </Text>
  </Flex>
);
