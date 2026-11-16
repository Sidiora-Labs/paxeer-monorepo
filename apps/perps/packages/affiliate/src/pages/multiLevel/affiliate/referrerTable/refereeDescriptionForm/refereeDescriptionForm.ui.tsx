import { useId, useState } from "react";
import surface from "../../../../../surface.module.css";
import { useTranslation } from "@paxeer-network/i18n";
import { Button, Divider, Flex, Text, formatAddress } from "@paxeer-network/ui";
import { RefereeDescriptionFormReturns } from "./refereeDescriptionForm.script";
import { RefereeDescriptionFormWidgetProps } from "./refereeDescriptionForm.widget";

export type RefereeDescriptionFormProps = RefereeDescriptionFormReturns &
  RefereeDescriptionFormWidgetProps;

export const RefereeDescriptionForm = (props: RefereeDescriptionFormProps) => {
  const { t } = useTranslation();
  const id = useId();
  const [touched, setTouched] = useState(false);
  const showError = touched && !props.isValid;

  const titleView = (
    <Flex
      width={"100%"}
      direction="column"
      itemAlign="start"
      className="oui-refereeDescriptionForm-header"
    >
      <Flex
        width="100%"
        height={"calc(var(--oui-space-1) * 13.25)"}
        itemAlign="center"
        className="oui-px-5 oui-pt-3"
      >
        <Text size="base" intensity={98} className="oui-leading-normal">
          {t("affiliate.editNote")}
        </Text>
      </Flex>
      <Divider intensity={8} className="oui-w-full" />
    </Flex>
  );

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        setTouched(true);
        if (props.isValid) void props.onConfirm();
      }}
    >
      <Flex
        direction="column"
        itemAlign="start"
        className={`${surface.surface} oui-affiliate-refereeDescriptionForm oui-font-semibold`}
      >
        {titleView}

        <Flex
          width={"100%"}
          direction="column"
          itemAlign="start"
          gap={6}
          className="oui-p-5"
        >
          <Flex width={"100%"} direction="column" itemAlign="start" gap={1}>
            <label htmlFor={id} className="oui-text-sm oui-text-content-muted">
              {t("affiliate.remark")}
            </label>
            <input
              id={id}
              name="description"
              autoComplete="on"
              maxLength={50}
              pattern="[a-zA-Z0-9@, _\-]*"
              onBlur={() => setTouched(true)}
              onInvalid={() => setTouched(true)}
              aria-invalid={showError}
              aria-describedby={`${id}-help`}
              autoFocus
              value={props.description}
              onChange={(event) => props.setDescription(event.target.value)}
              placeholder={t("affiliate.refereeNote.placeholder")}
              className={[
                "oui-h-10 oui-w-full oui-rounded-md oui-bg-base-6 oui-px-3 oui-py-2.5",
                "oui-text-sm oui-font-semibold oui-leading-normal oui-tracking-normal oui-text-base-contrast-98",
                "placeholder:oui-text-base-contrast-20",
                showError
                  ? "oui-outline oui-outline-1 oui-outline-danger"
                  : "oui-outline-none",
              ].join(" ")}
            />
            <Flex width="100%" gap={1} itemAlign="start" className="oui-pl-1">
              <span
                className={[
                  "oui-mt-2 oui-size-1 oui-shrink-0 oui-rounded-full",
                  showError ? "oui-bg-danger" : "oui-bg-base-contrast-54",
                ].join(" ")}
              />
              <Text
                id={`${id}-help`}
                role="status"
                size="2xs"
                intensity={showError ? 98 : 54}
                className={[
                  "oui-leading-normal",
                  showError ? "oui-text-danger" : undefined,
                ].join(" ")}
              >
                {showError
                  ? t("affiliate.refereeNote.helpText")
                  : t("affiliate.refereeNote.helpText.short")}
              </Text>
            </Flex>
          </Flex>

          <Flex
            width="100%"
            justify="between"
            itemAlign="center"
            gap={2}
            className="oui-text-2xs oui-font-semibold oui-leading-normal"
          >
            <Text intensity={54}>{t("affiliate.userAddress")}</Text>
            <Text
              intensity={98}
              className="oui-min-w-0 oui-flex-1 oui-text-end"
            >
              {formatAddress(props.address)}
            </Text>
          </Flex>
        </Flex>

        <Flex
          direction={"row"}
          gap={3}
          width={"100%"}
          className="oui-px-5 oui-pb-5 oui-pt-3"
        >
          <Button
            variant="contained"
            color="gray"
            fullWidth
            type="button"
            onClick={props.close}
            size="md"
            className="oui-refereeDescriptionForm-cancel-btn oui-rounded-md"
          >
            {t("common.cancel")}
          </Button>
          <Button
            fullWidth
            type="submit"
            disabled={props.buttonDisabled || props.confirmButtonLoading}
            loading={props.confirmButtonLoading}
            size="md"
            className="oui-refereeDescriptionForm-confirm-btn oui-rounded-md"
          >
            {t("common.save")}
          </Button>
        </Flex>
      </Flex>
    </form>
  );
};
