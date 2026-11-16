/* eslint-disable @typescript-eslint/no-explicit-any */
import React, { useMemo } from "react";
import { useTranslation } from "@paxeer-network/i18n";
import {
  Button,
  Flex,
  Text,
  TokenIcon,
  Tooltip,
  toast,
} from "@paxeer-network/ui";
import type { Column } from "@paxeer-network/ui";
import { getEffectiveConvertStatus } from "@paxeer-network/ui-transfer";
import { Decimal } from "@paxeer-network/utils";
import type { ConvertRecord, ConvertTransaction } from "../type";

export interface ConvertColumnsOptions {
  onDetailsClick?: (convertId: number) => void;
}

export interface ConvertDetailColumnsOptions {
  onTxClick?: (txId: string) => void;
  indexPrices: Record<string, number>;
  chainsInfo: any[];
}

export const ConvertStatusBadge = ({ status }: { status?: string }) => {
  const { t } = useTranslation();
  const normalizedStatus = status?.toLowerCase();
  const statusConfig = {
    completed: {
      color: "success" as const,
      label: t("assetHistory.status.completed"),
    },
    succeeded: {
      color: "success" as const,
      label: t("transfer.convert.succeeded"),
    },
    pending: {
      color: "warning" as const,
      label: t("assetHistory.status.pending"),
    },
    failed: {
      color: "danger" as const,
      label: t("assetHistory.status.failed"),
    },
    cancelled: {
      color: "neutral" as const,
      label: t("orders.status.canceled"),
    },
  };
  const config = statusConfig[normalizedStatus as keyof typeof statusConfig];

  return (
    <Text color={config?.color ?? "neutral"} size="xs" weight="semibold">
      {config?.label ||
        (normalizedStatus
          ? normalizedStatus.charAt(0).toUpperCase() + normalizedStatus.slice(1)
          : "-")}
    </Text>
  );
};

export const ConvertedAssetColumn = ({
  convertedAssets,
  details = [],
}: {
  convertedAssets?: Record<string, number> | null;
  details?: ConvertTransaction[];
}) => {
  const { t } = useTranslation();
  const assets = useMemo(() => {
    const convertedAssetNames = Object.keys(convertedAssets ?? {}).filter(
      Boolean,
    );

    // Failed conversions can have an empty `converted_asset` summary while
    // the asset name is still available on the transaction details.
    if (convertedAssetNames.length > 0) {
      return convertedAssetNames;
    }

    return Array.from(
      new Set(
        details
          .map((detail) => detail.converted_asset)
          .filter((asset): asset is string => Boolean(asset)),
      ),
    );
  }, [convertedAssets, details]);

  const quantities = useMemo<Record<string, number | string>>(() => {
    if (
      convertedAssets &&
      Object.keys(convertedAssets).some((asset) => Boolean(asset))
    ) {
      return convertedAssets;
    }

    return details.reduce<Record<string, number | string>>((result, detail) => {
      if (detail.converted_asset) {
        result[detail.converted_asset] = new Decimal(
          result[detail.converted_asset] ?? 0,
        )
          .plus(detail.converted_qty ?? 0)
          .toString();
      }
      return result;
    }, {});
  }, [convertedAssets, details]);

  const tooltipContent = useMemo(() => {
    return (
      <Flex
        direction="column"
        gap={2}
        className="oui-w-72 oui-font-semibold oui-text-base-contrast-80"
      >
        <Flex
          itemAlign="center"
          justify="between"
          className="oui-w-full oui-text-2xs oui-text-base-contrast-80"
        >
          <div>{t("common.assets")}</div>
          <div>{t("common.qty")}</div>
        </Flex>
        {assets.map((asset) => (
          <Flex
            key={asset}
            itemAlign="center"
            justify="between"
            className="oui-w-full"
          >
            <Flex itemAlign="center" gap={1}>
              <TokenIcon size="xs" name={asset} />
              <Text.formatted>{asset}</Text.formatted>
            </Flex>
            <div>
              <Text.formatted>{quantities[asset]}</Text.formatted>
            </div>
          </Flex>
        ))}
      </Flex>
    );
  }, [assets, quantities, t]);

  return (
    <Flex itemAlign="center" gap={2}>
      <div className="oui-relative oui-flex">
        {assets.slice(0, 3).map((asset, index) => (
          <div
            key={asset}
            className="oui-relative"
            style={{
              marginInlineStart:
                index > 0 ? "calc(-1 * var(--oui-space-2))" : 0,
              zIndex: assets.length + index,
            }}
          >
            <TokenIcon size="xs" name={asset} />
          </div>
        ))}
      </div>
      <Tooltip content={tooltipContent}>
        <Text.formatted className="oui-cursor-pointer oui-underline oui-decoration-line-16 oui-decoration-dashed oui-underline-offset-4">
          {assets.slice(0, 3).join(", ")}{" "}
          {assets.length > 3 && `+${assets.length - 3}`}
        </Text.formatted>
      </Tooltip>
    </Flex>
  );
};

export const useConvertColumns = (options: ConvertColumnsOptions) => {
  const { onDetailsClick } = options;
  const { t } = useTranslation();
  const onCopy = (e: any) => {
    e.preventDefault();
    e.stopPropagation();
    toast.success(t("common.copy.copied"));
  };

  const columns = React.useMemo<Column[]>(() => {
    return [
      {
        title: t("portfolio.overview.column.convert.convertedAsset"),
        dataIndex: "converted_asset",
        align: "left",

        maxWidth: 200,
        render(
          convertedAssets: Record<string, number> | null | undefined,
          record: ConvertRecord,
        ) {
          return (
            <ConvertedAssetColumn
              convertedAssets={convertedAssets}
              details={record.details}
            />
          );
        },
      },
      {
        title: t("portfolio.overview.column.convert.receivedAmount"),
        dataIndex: "received_qty",
        align: "left",

        render(qty: number, record: ConvertRecord) {
          return (
            <span>
              <Text.numeral dp={6} padding={false}>
                {qty}
              </Text.numeral>{" "}
              {record.received_asset}
            </span>
          );
        },
      },
      {
        title: t("common.fee"),
        dataIndex: "details",
        align: "left",

        render(details: ConvertRecord["details"], record: ConvertRecord) {
          const totalHaircut = details.reduce(
            (sum, detail) => sum + detail.haircut,
            0,
          );
          return (
            <span>
              <Text.numeral dp={6} padding={false}>
                {totalHaircut}
              </Text.numeral>{" "}
              {record.received_asset}
            </span>
          );
        },
      },
      {
        title: t("common.type"),
        dataIndex: "type",
        align: "left",

        render(type: string) {
          return <Text>{type.charAt(0).toUpperCase() + type.slice(1)}</Text>;
        },
      },
      {
        title: t("portfolio.overview.column.convert.convertId"),
        dataIndex: "convert_id",
        align: "left",

        render(convertId: number) {
          return (
            <Text.formatted onCopy={onCopy} copyable>
              {convertId}
            </Text.formatted>
          );
        },
      },
      {
        title: t("common.time"),
        dataIndex: "created_time",
        align: "left",

        rule: "date",
      },
      {
        title: t("common.status"),
        dataIndex: "status",
        align: "left",

        render(_status: string, record: ConvertRecord) {
          return (
            <ConvertStatusBadge status={getEffectiveConvertStatus(record)} />
          );
        },
      },
      {
        title: "",
        dataIndex: "updated_time",
        align: "center",

        render(convertId: number, record: ConvertRecord) {
          return (
            <Button
              size="sm"
              variant="text"
              className="oui-text-primary"
              onClick={() => onDetailsClick?.(record.convert_id)}
            >
              {t("portfolio.overview.column.convert.details")}
            </Button>
          );
        },
      },
    ];
  }, [onDetailsClick, t]);

  return columns;
};

// Converted asset, Qty., Received amount, Fee, TxID, Network, Status
export const useConvertDetailColumns = (
  options: ConvertDetailColumnsOptions,
) => {
  const { t } = useTranslation();
  const onCopy = (e: any) => {
    e.preventDefault();
    e.stopPropagation();
    toast.success(t("common.copy.copied"));
  };
  const columns = React.useMemo<Column[]>(() => {
    return [
      {
        title: t("portfolio.overview.column.convert.convertedAsset"),
        dataIndex: "converted_asset",
        align: "left",

        render(asset: string) {
          return (
            <Flex itemAlign="center" gap={2}>
              <TokenIcon name={asset} />
              {asset}
            </Flex>
          );
        },
      },
      {
        title: t("common.qty"),
        dataIndex: "converted_qty",
        align: "left",

        render(qty: number) {
          return (
            <Text.numeral dp={6} padding={false}>
              {qty}
            </Text.numeral>
          );
        },
      },
      {
        title: t("portfolio.overview.column.convert.receivedAmount"),
        dataIndex: "received_qty",
        align: "left",

        render(qty: number, detail: ConvertTransaction) {
          return (
            <span>
              <Text.numeral dp={6} padding={false}>
                {qty}
              </Text.numeral>{" "}
              {detail.received_asset}
            </span>
          );
        },
      },
      {
        title: t("common.fee"),
        dataIndex: "haircut",
        align: "left",

        render(haircut: number, detail: ConvertTransaction) {
          return (
            <span>
              <Text.numeral dp={6} padding={false}>
                {haircut}
              </Text.numeral>{" "}
              {detail.received_asset}
            </span>
          );
        },
      },
      {
        title: t("common.txId"),
        dataIndex: "tx_id",
        align: "left",

        render(txId: string, record: ConvertTransaction) {
          if (!txId) return <Text intensity={80}>-</Text>;
          const chainInfo = (options.chainsInfo as any[])?.find(
            (item) => record.chain_id === parseInt(item.chain_id),
          );
          const explorer_base_url = chainInfo?.explorer_base_url;
          const href = `${explorer_base_url}/tx/${txId}`;
          return (
            <a href={href} target="_blank" rel="noreferrer">
              <Text.formatted
                onCopy={onCopy}
                rule="txId"
                copyable={!!txId}
                className="oui-cursor-pointer oui-underline oui-decoration-line-16 oui-decoration-dashed oui-underline-offset-4"
              >
                {txId}
              </Text.formatted>
            </a>
          );
        },
      },
      {
        title: t("transfer.network"),
        dataIndex: "chain_id",
        align: "left",

        render(venue: string, record: ConvertTransaction) {
          return (
            <Text>
              {options.chainsInfo.find(
                (item) => item.chain_id == record.chain_id,
              )?.name || "-"}
            </Text>
          );
        },
      },
      {
        title: t("common.result"),
        dataIndex: "result",
        align: "left",

        render(result: string) {
          return <ConvertStatusBadge status={result} />;
        },
      },
    ];
  }, [t]);

  return columns;
};
