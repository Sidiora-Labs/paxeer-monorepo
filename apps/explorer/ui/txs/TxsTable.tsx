import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { TxsSocketType } from './socket/types';
import type { TransactionsSortingField, TransactionsSortingValue } from 'types/api/transaction';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { useMultichainContext } from 'lib/contexts/multichain';
import useInitialList from 'lib/hooks/useInitialList';
import useIsMobile from 'lib/hooks/useIsMobile';
import useLazyRenderedList from 'lib/hooks/useLazyRenderedList';
import { currencyUnits } from 'lib/units';
import { TableBody, TableColumnHeader, TableColumnHeaderSortable, TableHeader, TableRow } from 'toolkit/chakra/table';
import { LIST_TABLE_STYLES } from 'ui/shared/Page/ListPage';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import type { TxsTranslationQuery } from './noves/useDescribeTxs';
import TxsSocketNotice from './socket/TxsSocketNotice';
import type { TransactionFeedItem } from './TxPriceUpdate';
import TxsTableItem from './TxsTableItem';

type Props = {
  txs: Array<TransactionFeedItem>;
  sort: TransactionsSortingValue;
  onSortToggle?: (field: TransactionsSortingField) => void;
  top: number;
  showBlockInfo: boolean;
  socketType?: TxsSocketType;
  currentAddress?: string;
  enableTimeIncrement?: boolean;
  isLoading?: boolean;
  stickyHeader?: boolean;
  translationQuery?: TxsTranslationQuery;
};

const TxsTable = ({
  txs,
  sort,
  onSortToggle,
  top,
  showBlockInfo,
  socketType,
  currentAddress,
  enableTimeIncrement,
  isLoading,
  stickyHeader = true,
  translationQuery,
}: Props) => {
  const { cutRef, renderedItemsNum } = useLazyRenderedList(txs, !isLoading);
  const initialList = useInitialList({
    data: txs ?? [],
    idFn: (item) => item.hash,
    enabled: !isLoading,
  });
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;
  const isMobile = useIsMobile();

  const feeCurrency = config.UI.views.tx.hiddenFields?.fee_currency || config.chain.hasMultipleGasCurrencies ?
    '' :
    ' ' + currencyUnits.ether;

  const TableHeaderComponent = stickyHeader ? TableHeaderSticky : TableHeader;

  const showValue = !config.UI.views.tx.hiddenFields?.value;
  const showFee = !config.UI.views.tx.hiddenFields?.tx_fee;

  const columnNum = [ showBlockInfo, true, showValue, showFee ].filter(Boolean).length;
  const baseWidth = `${ 100 / columnNum }%`;

  return (
    <AddressHighlightProvider>
      <ActivityTable
        label="Transactions"
        variant="scan"
        css={{
          ...LIST_TABLE_STYLES,
          '& thead th': { whiteSpace: 'nowrap', verticalAlign: 'middle' },
        }}
        minWidth={{ base: 'var(--chakra-sizes-6xl)', lg: 'var(--chakra-sizes-5xl)' }}
      >
        <TableHeaderComponent top={ stickyHeader ? top : undefined }>
          <TableRow>
            <TableColumnHeader width="var(--chakra-spacing-12)"></TableColumnHeader>
            { chainData && <TableColumnHeader width="var(--chakra-spacing-8)"></TableColumnHeader> }
            <TableColumnHeader width="var(--chakra-spacing-48)">Transaction hash</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-37\.5)">Action</TableColumnHeader>
            { showBlockInfo && (
              onSortToggle ? (
                <TableColumnHeaderSortable
                  width="var(--chakra-spacing-28)"
                  isNumeric
                  sortField="block_number"
                  sortValue={ sort }
                  onSortToggle={ onSortToggle }
                >
                  Block
                </TableColumnHeaderSortable>
              ) : (
                <TableColumnHeader width="var(--chakra-spacing-28)" isNumeric>Block</TableColumnHeader>
              )
            ) }
            <TableColumnHeader width="var(--chakra-spacing-32)">
              <Flex flexDirection="row" alignItems="center" flexWrap="nowrap" whiteSpace="nowrap">
                Age
                <TimeFormatToggle flexShrink={ 0 }/>
              </Flex>
            </TableColumnHeader>
            <TableColumnHeader width={ columnNum <= 2 ? baseWidth : 'var(--chakra-spacing-75)' }>From / To</TableColumnHeader>
            { showValue && (
              onSortToggle ? (
                <TableColumnHeaderSortable
                  width={ baseWidth }
                  isNumeric
                  sortField="value"
                  sortValue={ sort }
                  onSortToggle={ onSortToggle }
                >
                  { `Amount ${ currencyUnits.ether }` }
                </TableColumnHeaderSortable>
              ) : (
                <TableColumnHeader width={ baseWidth } isNumeric>Amount</TableColumnHeader>
              )
            ) }
            { showFee && (
              onSortToggle ? (
                <TableColumnHeaderSortable
                  width={ baseWidth }
                  isNumeric
                  pr={ 5 }
                  sortField="fee"
                  sortValue={ sort }
                  onSortToggle={ onSortToggle }
                >
                  { `Txn fee${ feeCurrency }` }
                </TableColumnHeaderSortable>
              ) : (
                <TableColumnHeader width={ baseWidth } isNumeric pr={ 5 }>Txn fee</TableColumnHeader>
              )
            ) }
          </TableRow>
        </TableHeaderComponent>
        <TableBody>
          { socketType && <TxsSocketNotice type={ socketType } place="table" isLoading={ isLoading }/> }
          { txs.slice(0, renderedItemsNum).map((item, index) => {
            return (
              <TxsTableItem
                key={ item.hash + (isLoading ? index : '') }
                tx={ item }
                showBlockInfo={ showBlockInfo }
                currentAddress={ currentAddress }
                enableTimeIncrement={ enableTimeIncrement }
                isLoading={ isLoading }
                animation={ initialList.getAnimationProp(item) }
                chainData={ chainData }
                translationIsLoading={ translationQuery?.isLoading }
                translationData={ translationQuery?.data?.find(({ txHash }) => txHash.toLowerCase() === item.hash.toLowerCase()) }
                isMobile={ isMobile }
              />
            );
          }) }
          { renderedItemsNum < txs.length && <TableRow ref={ cutRef }/> }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default React.memo(TxsTable);
