import { useTranslation } from "@paxeer-network/i18n";
import { FC } from "react";
import { EditIcon } from "../icons/editIcon";

export const EditCode: FC<{
  size?: number;
  onClick?: () => void;
}> = (props) => {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-label={t("common.edit")}
      onClick={props.onClick}
      className="oui-min-h-12 oui-min-w-12 oui-rounded-md hover:oui-bg-base-5"
    >
      <EditIcon
        className=" oui-mt-px oui-cursor-pointer oui-fill-base-contrast-36 hover:oui-fill-base-contrast-80"
        fillOpacity={1}
        fill="currentColor"
      />
    </button>
  );
};
