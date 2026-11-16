import { ApiDialog } from "./apiDialog";
import styles from "../../../presentation.module.css";
import { FC, useEffect, useState, useId } from "react";
import { ScopeType } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { cn, Flex, Statistic, Text } from "@paxeer-network/ui";
import { ApiManagerScriptReturns } from "../apiManager.script";

export const CreateAPIKeyDialog: FC<ApiManagerScriptReturns> = (props) => {
  const [ipText, setIpText] = useState("");
  const [read, setRead] = useState(true);
  const [trade, setTrade] = useState(true);
  const [hint, setHint] = useState("");
  const { t } = useTranslation();
  const inputId = useId();

  useEffect(() => {
    if (!props.showCreateDialog) {
      setIpText("");
      setRead(true);
      setTrade(true);
      setHint("");
    }
  }, [props.showCreateDialog]);

  useEffect(() => {
    if (ipText.length === 0) setHint("");
  }, [ipText]);

  return (
    <ApiDialog
      size="sm"
      open={props.showCreateDialog}
      onOpenChange={(open) => {
        props.hideCreateDialog?.();
      }}
      title={t("portfolio.apiKey.create.dialog.title")}
      actions={{
        primary: {
          label: t("common.confirm"),
          className: "oui-w-28 lg:oui-w-40",
          "data-testid": "oui-testid-apiKey-createApiKey-dialog-comfirm-btn",
          onClick: async () => {
            if (ipText.length > 0) {
              const hint = props.verifyIP(ipText);
              setHint(hint);
              if (hint.length > 0) {
                return;
              }
            }

            const scopes: string[] = [];
            if (read) {
              scopes.push("read");
            }
            if (trade) {
              scopes.push("trading");
            }
            await props.doCreate(ipText, scopes.join(",") as ScopeType);
          },
          disabled: !trade && !read,
          size: "md",
        },
      }}
      classNames={{
        footer: "oui-justify-center",
        content: cn(
          styles.surface,
          "oui-bg-base-8 oui-w-72 lg:oui-w-96 oui-max-w-[calc(100vw-var(--oui-space-6))] oui-font-semibold",
        ),
      }}
    >
      <Flex direction={"column"} gap={6}>
        <Flex direction={"column"} gap={1} width={"100%"} itemAlign={"start"}>
          <label
            htmlFor={inputId}
            className="oui-text-2xs oui-text-base-contrast-80"
          >
            {t("portfolio.apiKey.create.ipRestriction")}
          </label>
          <textarea
            autoFocus
            id={inputId}
            name="ip-restriction"
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            aria-invalid={hint.length > 0}
            aria-describedby={hint ? `${inputId}-hint` : undefined}
            onBlur={() => setHint(ipText ? props.verifyIP(ipText) : "")}
            data-testid="oui-testid-apiKey-createApiKey-dialog-textarea"
            placeholder={t("portfolio.apiKey.create.ipRestriction.placeholder")}
            className={cn(
              styles.textarea,
              "oui-min-h-24 oui-w-full oui-rounded-xl oui-bg-base-6 oui-p-3 oui-text-sm oui-text-base-contrast-80",
              "oui-border-0 oui-outline-none focus:oui-border-2 focus:oui-border-primary-darken",
              "placeholder:oui-text-base-contrast-80",
              hint.length > 0 &&
                "oui-outline-1 oui-outline-danger focus:oui-outline-none",
            )}
            value={ipText}
            onChange={(e) => {
              setIpText(e.target.value);
              setHint("");
            }}
          />
          {hint.length > 0 && (
            <Flex id={`${inputId}-hint`} gap={1} className="oui-relative">
              <div
                className={cn(
                  "oui-absolute oui-top-2.5",
                  "oui-size-1 oui-rounded-full oui-bg-danger",
                )}
              />
              <Text color="danger" size="xs" className="oui-ms-2">
                {hint}
              </Text>
            </Flex>
          )}
        </Flex>
        <Statistic
          label={
            <Text size="xs" intensity={80}>
              {t("portfolio.apiKey.permissions")}
            </Text>
          }
          className="oui-w-full"
        >
          <Flex
            direction={"row"}
            gap={6}
            itemAlign={"start"}
            className={`${styles.permissions} oui-mt-2`}
          >
            <Checkbox
              size={18}
              checked={read}
              onCheckedChange={(e) => setRead(e as boolean)}
              label={t("portfolio.apiKey.permissions.read")}
              testid="oui-testid-apiKey-createApiKey-dialog-read-checkbox"
            />
            <Checkbox
              size={18}
              checked={trade}
              onCheckedChange={(e) => setTrade(e as boolean)}
              label={t("portfolio.apiKey.permissions.trading")}
              testid="oui-testid-apiKey-createApiKey-dialog-trading-checkbox"
            />
          </Flex>
        </Statistic>
      </Flex>
    </ApiDialog>
  );
};

export const Checkbox: FC<{
  size?: number;
  checked: boolean;
  onCheckedChange: (checked?: boolean) => void;
  disabled?: boolean;
  label: string;
  testid?: string;
}> = (props) => {
  return (
    <label className={styles.permission}>
      <input
        type="checkbox"
        name="api-permissions"
        value={props.label}
        checked={props.checked}
        disabled={props.disabled}
        onChange={(event) => props.onCheckedChange(event.target.checked)}
        data-testid={props.testid}
        className="oui-size-5 oui-accent-primary"
      />
      {props.label}
    </label>
  );
};
