import React, { useEffect } from "react";
import { useAccount } from "@paxeer-network/hooks";
import { useTranslation } from "@paxeer-network/i18n";
import { AssetHistorySideEnum } from "@paxeer-network/types";
import {
  ArrowDownSquareFillIcon,
  ArrowLeftRightSquareFill,
  ArrowUpSquareFillIcon,
  Card,
  FeeTierIcon,
  ServerFillIcon,
  TabPanel,
  Tabs,
  VaultsIcon,
} from "@paxeer-network/ui";
import type { TabName } from "./historyDataGroup.script";

import { AssetHistoryWidget } from "../assetHistory";

import { FundingHistoryWidget } from "../funding";

import { DistributionHistoryWidget } from "../distribution";

import { TransferHistoryWidget } from "../TransferHistory";

import { ConvertHistoryWidget } from "../../assets/convertPage/convert.widget";

import { VaultsHistoryWidget } from "../VaultsHistory";

export const HistoryDataGroupDesktop: React.FC<{
  active?: TabName;
  onTabChange: (tab: string) => void;
}> = (props) => {
  const { active = "deposit", onTabChange } = props;
  const { t } = useTranslation();
  const { isMainAccount } = useAccount();
  useEffect(() => {
    if (active === "vaults" && !isMainAccount) {
      onTabChange("deposit");
    }
  }, [active, isMainAccount]);
  return (
    <Card>
      <Tabs
        value={active}
        onValueChange={onTabChange}
        variant="contained"
        size="xl"
      >
        <TabPanel
          title={t("common.deposits")}
          icon={<ArrowDownSquareFillIcon />}
          value={"deposit"}
        >
          <React.Suspense fallback={null}>
            <AssetHistoryWidget side={AssetHistorySideEnum.DEPOSIT} />
          </React.Suspense>
        </TabPanel>
        <TabPanel
          title={t("common.withdrawals")}
          icon={<ArrowUpSquareFillIcon />}
          value={"withdraw"}
        >
          <React.Suspense fallback={null}>
            <AssetHistoryWidget side={AssetHistorySideEnum.WITHDRAW} />
          </React.Suspense>
        </TabPanel>
        <TabPanel
          title={t("common.funding")}
          icon={<FeeTierIcon />}
          value={"funding"}
        >
          <React.Suspense fallback={null}>
            <FundingHistoryWidget />
          </React.Suspense>
        </TabPanel>
        <TabPanel
          title={t("portfolio.overview.distribution")}
          icon={<ServerFillIcon />}
          value={"distribution"}
        >
          <React.Suspense fallback={null}>
            <DistributionHistoryWidget />
          </React.Suspense>
        </TabPanel>
        <TabPanel
          title={t("portfolio.overview.transferHistory")}
          icon={<ArrowLeftRightSquareFill />}
          value={"transfer"}
        >
          <React.Suspense fallback={null}>
            <TransferHistoryWidget />
          </React.Suspense>
        </TabPanel>
        <TabPanel
          title={t("portfolio.overview.tab.convert.history")}
          value={"convert"}
        >
          <React.Suspense fallback={null}>
            <ConvertHistoryWidget />
          </React.Suspense>
        </TabPanel>
        {isMainAccount && (
          <TabPanel
            value={"vaults"}
            icon={<VaultsIcon />}
            title={t("portfolio.overview.vaults")}
          >
            <React.Suspense fallback={null}>
              <VaultsHistoryWidget />
            </React.Suspense>
          </TabPanel>
        )}
      </Tabs>
    </Card>
  );
};
