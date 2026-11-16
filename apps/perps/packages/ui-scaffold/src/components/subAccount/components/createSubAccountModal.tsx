import { Tooltip } from "../../../../../ui-connector/src/platformOverlays";
import { SimpleDialog } from "../../../../../ui-connector/src/platformOverlays";
import { useState, useMemo } from "react";
import { useAccount } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { useAppContext } from "@paxeer-network/react-app";
import { Button, Flex, toast, Text, useScreen, cn } from "@paxeer-network/ui";
import { AddIcon } from "../icons";
import { NickNameTextField } from "./common";

const MAX_SUB_ACCOUNT_COUNT = 10;
type CreateSubAccountProps = {
  create: (nickName: string) => Promise<void>;
};

export const CreateSubAccount = (props: CreateSubAccountProps) => {
  const { t } = useTranslation();
  const { isMobile } = useScreen();
  const [open, setOpen] = useState(false);
  const [nickName, setNickName] = useState<string | undefined>(undefined);
  const { state } = useAccount();
  const [invalid, setInvalid] = useState(false);
  const [loading, setLoading] = useState(false);

  const { widgetConfigs } = useAppContext();

  const maxSubAccountCount = useMemo(() => {
    return (
      widgetConfigs?.subAccount?.maxSubAccountCount ?? MAX_SUB_ACCOUNT_COUNT
    );
  }, [widgetConfigs]);

  const subAccountCount = useMemo(() => {
    return state.subAccounts?.length ?? 0;
  }, [state]);

  const trigger = useMemo(() => {
    return subAccountCount >= maxSubAccountCount ? (
      <Tooltip
        className="oui-max-w-48"
        content={t("subAccount.modal.create.max.description")}
      >
        <button
          type="button"
          className="oui-shell-icon-button oui-cursor-not-allowed"
          aria-disabled="true"
          aria-label={t("subAccount.modal.create.max.description")}
        >
          <AddIcon className="oui-fill-base-contrast-80" />
        </button>
      </Tooltip>
    ) : (
      <button
        type="button"
        className="oui-shell-icon-button"
        aria-label="Create sub-account"
        onClick={() => {
          setOpen(true);
        }}
      >
        <AddIcon
          className={cn(
            "oui-cursor-pointer oui-fill-base-contrast-54 hover:oui-fill-base-contrast",
          )}
        />
      </button>
    );
  }, [subAccountCount, maxSubAccountCount, t]);

  const header = (
    <Flex
      py={3}
      direction="column"
      justify="between"
      itemAlign="start"
      width="100%"
    >
      <Text weight="semibold">{t("subAccount.modal.create.title")}</Text>
      <Text className="oui-text-2xs oui-text-base-contrast-80">
        {t("subAccount.modal.create.description", {
          subAccountCount,
          remainingCount: maxSubAccountCount - subAccountCount,
        })}
      </Text>
    </Flex>
  );

  const reset = () => {
    setNickName("");
    setInvalid(false);
    setLoading(false);
  };

  const validateNickName = (nickName: string | undefined) => {
    if (!nickName || !(nickName.length >= 1 && nickName.length <= 20)) {
      setInvalid(true);
      return true;
    }
    setInvalid(false);
    return false;
  };

  const doCreatSubAccount = (nickName: string | undefined) => {
    let _nickName = `Sub-account ${subAccountCount + 1}`;
    if (nickName) {
      _nickName = nickName.trim();
    }

    setLoading(true);
    props
      .create(_nickName)
      .then((res) => {
        console.log("res", res);
        reset();
        toast.success(t("subAccount.modal.create.success.description"));

        setOpen(false);
      })
      .catch((e: any) => {
        toast.error(t("subAccount.modal.create.failed.description"));
      })
      .finally(() => {
        setLoading(false);
      });
  };
  return (
    <>
      {trigger}
      <SimpleDialog
        title={header}
        open={open}
        onOpenChange={(open) => {
          reset();
          setOpen(open);
        }}
        size={isMobile ? "sm" : "xl"}
        classNames={{
          content: "oui-shell-overlay oui-w-96",
        }}
      >
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (!loading) {
              (() => {
                const invalid = validateNickName(nickName);
                if (invalid) {
                  return;
                }
                doCreatSubAccount(nickName);
              })();
            }
          }}
          className="oui-flex oui-flex-col oui-gap-4"
        >
          <NickNameTextField
            nickName={nickName}
            setNickName={(nickName) => {
              if (invalid) validateNickName(nickName);
              setNickName(nickName);
            }}
            subAccountCount={subAccountCount}
            onBlur={() => {
              validateNickName(nickName);
            }}
            invalid={invalid}
          />
          <div className="oui-flex oui-flex-wrap oui-gap-3">
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
    </>
  );
};
