import { SimpleDialog } from "../../../../../ui-connector/src/platformOverlays";
import { useEffect, useState } from "react";
import { useAccount } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, toast, Text } from "@paxeer-network/ui";
import { NickNameTextField } from "./common";

export const EditNickNameDialog = (props: {
  nickName: string;
  open: boolean;
  accountId: string;
  onOpenChange: (open: boolean) => void;
}) => {
  const { subAccount } = useAccount();
  const [loading, setLoading] = useState(false);
  const { t } = useTranslation();
  const [newNickName, setNewNickName] = useState<string | undefined>(undefined);
  const [invalid, setInvalid] = useState(false);

  const validateNickName = (nickName: string | undefined) => {
    if (!nickName || !(nickName.length >= 1 && nickName.length <= 20)) {
      setInvalid(true);
      return true;
    }
    setInvalid(false);
  };
  useEffect(() => {
    setNewNickName(props.nickName);
    setInvalid(false);
    setLoading(false);
  }, [props.nickName, props.open]);
  return (
    <SimpleDialog
      title={<Text>{t("subAccount.modal.edit.title")}</Text>}
      open={props.open}
      onOpenChange={props.onOpenChange}
      classNames={{
        content: "oui-shell-overlay oui-w-96",
      }}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!loading) {
            (() => {
              if (validateNickName(newNickName)) {
                return;
              }
              setLoading(true);
              subAccount
                ?.update({
                  subAccountId: props.accountId,
                  description: newNickName,
                })
                .catch((e) => {
                  console.log("e", e);
                  toast.error(t("subAccount.modal.edit.failed.description"));
                })
                .then((res) => {
                  toast.success(t("subAccount.modal.edit.success.description"));
                  props.onOpenChange(false);
                })
                .finally(() => {
                  setLoading(false);
                });
            })();
          }
        }}
        className="oui-flex oui-flex-col oui-gap-4"
      >
        <NickNameTextField
          nickName={newNickName}
          setNickName={(nickName) => {
            if (invalid) validateNickName(nickName);
            setNewNickName(nickName ?? "");
          }}
          onBlur={() => {
            validateNickName(newNickName);
          }}
          invalid={invalid}
        />
        <div className="oui-flex oui-flex-wrap oui-gap-3">
          <Button
            type="button"
            color="gray"
            onClick={() => props.onOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            loading={loading}
            disabled={loading || invalid}
            fullWidth
          >
            {t("common.confirm")}
          </Button>
        </div>
      </form>
    </SimpleDialog>
  );
};
