import { capitalize } from 'es-toolkit';
import React from 'react';

import type { Block } from 'types/api/block';
import type { ClusterChainConfig } from 'types/multichain';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import useInitialList from 'lib/hooks/useInitialList';
import getNetworkValidatorTitle from 'lib/networks/getNetworkValidatorTitle';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import BlocksTableItem from 'ui/blocks/BlocksTableItem';
import { LIST_TABLE_STYLES } from 'ui/shared/Page/ListPage';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';

interface Props {
  data: Array<Block>;
  isLoading?: boolean;
  top: number;
  page: number;
  socketInfoNum?: number;
  showSocketErrorAlert?: boolean;
  showSocketInfo?: boolean;
  chainData?: ClusterChainConfig;
}

const VALIDATOR_COL_WEIGHT = 23;
const GAS_COL_WEIGHT = 33;
const REWARD_COL_WEIGHT = 22;
const FEES_COL_WEIGHT = 22;

const BlocksTable = ({ data, isLoading, top, page, showSocketInfo, socketInfoNum, showSocketErrorAlert, chainData }: Props) => {
  const appConfig = chainData?.app_config ?? config;
  const currency = appConfig.chain.currency;
  const isRollup = appConfig.features.rollup.isEnabled;
  const initialList = useInitialList({
    data: data ?? [],
    idFn: (item) => item.hash,
    enabled: !isLoading,
  });

  const widthBase =
    (!appConfig.UI.views.block.hiddenFields?.miner ? VALIDATOR_COL_WEIGHT : 0) +
    GAS_COL_WEIGHT +
    (!isRollup && !appConfig.UI.views.block.hiddenFields?.total_reward ? REWARD_COL_WEIGHT : 0) +
    (!isRollup && !appConfig.UI.views.block.hiddenFields?.burnt_fees ? FEES_COL_WEIGHT : 0);

  return (
    <AddressHighlightProvider>
      <TableRoot variant="scan" css={ LIST_TABLE_STYLES } minWidth={ chainData ? '1100px' : '1070px' } fontWeight={ 500 }>
        <TableHeaderSticky top={ top }>
          <TableRow>
            { chainData && <TableColumnHeader width="38px"/> }
            <TableColumnHeader width="180px">
              Block
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader width="120px" isNumeric>Size, bytes</TableColumnHeader>
            { !appConfig.UI.views.block.hiddenFields?.miner && (
              <TableColumnHeader width={ `${ VALIDATOR_COL_WEIGHT / widthBase * 100 }%` } minW="160px">
                { capitalize(getNetworkValidatorTitle()) }
              </TableColumnHeader>
            ) }
            <TableColumnHeader width="64px" isNumeric>Txn</TableColumnHeader>
            <TableColumnHeader width={ `${ GAS_COL_WEIGHT / widthBase * 100 }%` } isNumeric>Gas used</TableColumnHeader>
            { !isRollup && !appConfig.UI.views.block.hiddenFields?.total_reward &&
              <TableColumnHeader width={ `${ REWARD_COL_WEIGHT / widthBase * 100 }%` } isNumeric>Reward { currency.symbol || 'ETH' }</TableColumnHeader> }
            { !isRollup && !appConfig.UI.views.block.hiddenFields?.burnt_fees &&
              <TableColumnHeader width={ `${ FEES_COL_WEIGHT / widthBase * 100 }%` } isNumeric>Burnt fees { currency.symbol || 'ETH' }</TableColumnHeader> }
            { !isRollup && !appConfig.UI.views.block.hiddenFields?.base_fee &&
              <TableColumnHeader width="150px" isNumeric>Base fee</TableColumnHeader> }
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { showSocketInfo && (
            <SocketNewItemsNotice.Desktop
              showErrorAlert={ showSocketErrorAlert }
              num={ socketInfoNum }
              type="block"
              isLoading={ isLoading }
            />
          ) }
          { data.map((item, index) => (
            <BlocksTableItem
              key={ item.hash + (isLoading ? `${ index }_${ page }` : '') }
              data={ item }
              enableTimeIncrement={ page === 1 && !isLoading }
              isLoading={ isLoading }
              animation={ initialList.getAnimationProp(item) }
              chainData={ chainData }
              appConfig={ appConfig }
            />
          )) }
        </TableBody>
      </TableRoot>
    </AddressHighlightProvider>
  );
};

export default BlocksTable;
