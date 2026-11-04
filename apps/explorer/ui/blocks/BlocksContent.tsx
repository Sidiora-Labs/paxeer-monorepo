import { Box } from '@chakra-ui/react';
import React from 'react';

import type { BlockType } from 'types/api/block';

import { route } from 'nextjs/routes';

import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import useIsMobile from 'lib/hooks/useIsMobile';
import { HOMEPAGE_STATS } from 'stubs/stats';
import { Link } from 'toolkit/chakra/link';
import BlocksList from 'ui/blocks/BlocksList';
import BlocksTable from 'ui/blocks/BlocksTable';
import ActionBar from 'ui/shared/ActionBar';
import DataListDisplay from 'ui/shared/DataListDisplay';
import IconSvg from 'ui/shared/IconSvg';
import Pagination from 'ui/shared/pagination/Pagination';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import { formatScanTableCount, ScanTableCard } from 'ui/shared/scan';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';

import useBlocksNewBlock from './useBlocksNewBlock';

const TABS_HEIGHT = 88;
const ITEMS_NAME = 'blocks';

export interface Props {
  type?: BlockType;
  query: QueryWithPagesResult<'general:blocks'> | QueryWithPagesResult<'general:optimistic_l2_txn_batch_blocks'>;
  enableSocket?: boolean;
  top?: number;
  isInsideTableCard?: boolean;
}

const BlocksContent = ({ type, query, enableSocket = true, top, isInsideTableCard = false }: Props) => {
  const isMobile = useIsMobile();
  const multichainContext = useMultichainContext();
  const { newItemsCount, showSocketAlert } = useBlocksNewBlock({
    type,
    isDisabled: query.isPlaceholderData || query.isError || query.pagination.page !== 1 || !enableSocket,
  });

  const chainData = multichainContext?.chain;

  const isMainList = (!type || type === 'block') && enableSocket;

  const statsQuery = useApiQuery('general:stats', {
    queryOptions: {
      enabled: isMainList,
      placeholderData: isMainList ? HOMEPAGE_STATS : undefined,
    },
  });

  const content = query.data?.items ? (
    <>
      <Box hideFrom="lg">
        { query.pagination.page === 1 && enableSocket && (
          <SocketNewItemsNotice.Mobile
            num={ newItemsCount }
            showErrorAlert={ showSocketAlert }
            type="block"
            isLoading={ query.isPlaceholderData }
          />
        ) }
        <BlocksList data={ query.data.items } isLoading={ query.isPlaceholderData } page={ query.pagination.page } chainData={ chainData }/>
      </Box>
      <Box hideBelow="lg">
        <BlocksTable
          data={ query.data.items }
          top={ top || (query.pagination.isVisible ? TABS_HEIGHT : 0) }
          page={ query.pagination.page }
          isLoading={ query.isPlaceholderData }
          showSocketInfo={ query.pagination.page === 1 && enableSocket }
          socketInfoNum={ newItemsCount }
          showSocketErrorAlert={ showSocketAlert }
          chainData={ chainData }
        />
      </Box>
    </>
  ) : null;

  const actionBar = isMobile ? (
    <ActionBar mt={ -6 }>
      <Link href={ route({ pathname: '/block/countdown' }, multichainContext) }>
        <IconSvg name="hourglass" boxSize={ 5 } mr={ 2 }/>
        <span>Block countdown</span>
      </Link>
      <Pagination ml="auto" { ...query.pagination }/>
    </ActionBar>
  ) : null;

  const paginationNode = !isInsideTableCard && !isMobile && query.pagination.isVisible ? <Pagination { ...query.pagination }/> : null;

  const items = query.data?.items;
  const totalBlocks = Number(statsQuery.data?.total_blocks);

  const countLine = (() => {
    if (!items || items.length === 0) {
      return 'Blocks';
    }

    if (isMainList && Number.isFinite(totalBlocks) && totalBlocks >= items.length) {
      return formatScanTableCount({ kind: 'latest', value: totalBlocks, shownValue: items.length, itemsName: ITEMS_NAME });
    }

    if (query.pagination.hasNextPage || query.pagination.page > 1) {
      return formatScanTableCount({ kind: 'more_than', value: query.pagination.page * items.length, itemsName: ITEMS_NAME });
    }

    return formatScanTableCount({ kind: 'total', value: items.length, itemsName: ITEMS_NAME });
  })();

  const note = (() => {
    switch (type) {
      case 'reorg':
        return 'Blocks replaced by a competing block at the same height';
      case 'uncle':
        return 'Blocks that were produced but left out of the canonical chain';
      default:
        return `Showing page ${ query.pagination.page } of the records the node returns, newest first`;
    }
  })();

  return (
    <DataListDisplay
      isError={ query.isError }
      itemsNum={ query.data?.items?.length }
      emptyText="There are no blocks."
      actionBar={ actionBar }
    >
      { content && (isInsideTableCard ? content : (
        <ScanTableCard
          title={ countLine }
          note={ note }
          pagination={ paginationNode }
        >
          { content }
        </ScanTableCard>
      )) }
    </DataListDisplay>
  );
};

export default React.memo(BlocksContent);
