import { FC } from "react";
import { Trans } from "@paxeer-network/i18n";
import { Flex, Text } from "@paxeer-network/ui";

export const ViewFAQs: FC = () => {
  return (
    <Flex justify="center" gapX={1} mt={3}>
      <Trans
        i18nKey="transfer.swapDeposit.viewFAQs"
        components={[
          <a
            href="https://learn.woo.org/woofi/faqs/woofi-pro"
            target="_blank"
            rel="noreferrer"
            className="oui-inline-flex oui-min-h-12 oui-items-center oui-text-xs oui-text-primary-light"
            key="0"
          />,
        ]}
      />
    </Flex>
  );
};
