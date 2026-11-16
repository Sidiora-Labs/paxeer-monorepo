import { ApiDialog } from "./apiDialog";
import { FC } from "react";
import { Flex, Text } from "@paxeer-network/ui";
import { APIKeyItem } from "@paxeer-network/hooks";
import { formatKey } from "../apiManager.ui";
import { useTranslation, Trans } from "@paxeer-network/i18n";

export const DeleteAPIKeyDialog: FC<{
  item: APIKeyItem;
  open: boolean;
  setOpen?: any;
  onDelete?: (item: APIKeyItem) => Promise<void>;
}> = (props) => {
  const { item, open, setOpen, onDelete } = props;
  const { t } = useTranslation();
  return (
    <ApiDialog
      size="sm"
      open={open}
      onOpenChange={setOpen}
      title={t("portfolio.apiKey.delete.dialog.title")}
      actions={{
        primary: {
          label: t("common.confirm"),
          "data-testid": "oui-testid-apiKey-deleteApiKey-dialog-confirm-btn",
          className: "oui-w-28 lg:oui-w-40",
          size: "md",
          onClick: async () => {
            await props.onDelete?.(item);
            setOpen(false);
          },
        },
        secondary: {
          label: t("common.cancel"),
          className: "oui-w-28 lg:oui-w-40",
          size: "md",
          onClick: async () => {
            setOpen(false);
          },
        },
      }}
      classNames={{
        footer: "oui-justify-center",
        content: "oui-bg-base-8 oui-w-72 lg:oui-w-96 oui-font-semibold",
      }}
    >
      <Flex className="oui-text-xs">
        {/* @ts-ignore */}
        <Trans
          i18nKey="portfolio.apiKey.delete.dialog.description"
          values={{ apiKey: formatKey(item?.paxeerNetwork_key) }}
          components={[<Text color="primary" className="oui-px-1" />]}
        />
      </Flex>
    </ApiDialog>
  );
};
