import { MouseEventHandler } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import { CopyIcon, toast } from "@paxeer-network/ui";

export function CopyButton({
  value,
  onCopy,
}: {
  value: string;
  onCopy?: MouseEventHandler<HTMLButtonElement>;
}) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      aria-label={`${t("common.copy")} ${value}`}
      className="oui-inline-flex oui-min-h-12 oui-min-w-12 oui-shrink-0 oui-items-center oui-justify-center oui-rounded-md"
      onClick={async (event) => {
        event.preventDefault();
        event.stopPropagation();
        if (onCopy) {
          onCopy(event);
          return;
        }
        try {
          await navigator.clipboard.writeText(value);
          toast.success(t("common.copy.copied"));
        } catch {
          toast.error(t("common.copy.failed"));
        }
      }}
    >
      <CopyIcon size={16} color="currentColor" />
    </button>
  );
}
