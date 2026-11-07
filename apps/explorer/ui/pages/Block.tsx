import { chakra, Flex } from '@chakra-ui/react';
import { capitalize } from 'es-toolkit';
import { useRouter } from 'next/router';
import React from 'react';

import type { TabItemRegular } from 'toolkit/components/AdaptiveTabs/types';

import { route, routeParams } from 'nextjs/routes';

import config from 'configs/app';
import { useMultichainContext } from 'lib/contexts/multichain';
import throwOnAbsentParamError from 'lib/errors/throwOnAbsentParamError';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import getNetworkValidatorTitle from 'lib/networks/getNetworkValidatorTitle';
import getQueryParamString from 'lib/router/getQueryParamString';
import { Badge } from 'toolkit/chakra/badge';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import RoutedTabs from 'toolkit/components/RoutedTabs/RoutedTabs';
import BlockCeloEpochTag from 'ui/block/BlockCeloEpochTag';
import BlockDeposits from 'ui/block/BlockDeposits';
import BlockDetails from 'ui/block/BlockDetails';
import BlockInternalTxs from 'ui/block/BlockInternalTxs';
import BlockWithdrawals from 'ui/block/BlockWithdrawals';
import useBlockBlobTxsQuery from 'ui/block/useBlockBlobTxsQuery';
import useBlockDepositsQuery from 'ui/block/useBlockDepositsQuery';
import useBlockInternalTxsQuery from 'ui/block/useBlockInternalTxsQuery';
import useBlockQuery from 'ui/block/useBlockQuery';
import useBlockTxsQuery from 'ui/block/useBlockTxsQuery';
import useBlockWithdrawalsQuery from 'ui/block/useBlockWithdrawalsQuery';
import TextAd from 'ui/shared/ad/TextAd';
import ServiceDegradationWarning from 'ui/shared/alerts/ServiceDegradationWarning';
import BlockPendingUpdateAlert from 'ui/shared/block/BlockPendingUpdateAlert';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import * as BlockEntity from 'ui/shared/entities/block/BlockEntity';
import IconSvg from 'ui/shared/IconSvg';
import NetworkExplorers from 'ui/shared/NetworkExplorers';
import PageTitle from 'ui/shared/Page/PageTitle';
import { entityTabGroup } from 'ui/shared/RoutedTabs/EntityTabs';
import TxsWithFrontendSorting from 'ui/txs/TxsWithFrontendSorting';

type BlockTab = TabItemRegular;

const beaconChainFeature = config.features.beaconChain;
const apiDocsFeature = config.features.apiDocs;

const BlockPageContent = () => {
  const router = useRouter();
  const heightOrHash = getQueryParamString(router.query.height_or_hash);
  const tab = getQueryParamString(router.query.tab) || 'txs';
  const multichainContext = useMultichainContext();

  const blockQuery = useBlockQuery({ heightOrHash });
  const blockTxsQuery = useBlockTxsQuery({ heightOrHash, blockQuery, tab });
  const blockWithdrawalsQuery = useBlockWithdrawalsQuery({ heightOrHash, blockQuery, tab });
  const blockDepositsQuery = useBlockDepositsQuery({ heightOrHash, blockQuery, tab });
  const blockBlobTxsQuery = useBlockBlobTxsQuery({ heightOrHash, blockQuery, tab });
  const blockInternalTxsQuery = useBlockInternalTxsQuery({ heightOrHash, blockQuery, tab });

  const tabs: Array<BlockTab> = React.useMemo(() => ([
    {
      id: 'index',
      title: 'Details',
      component: (
        <>
          <Flex rowGap={{ base: 1, lg: 2 }} mb={{ base: 3, lg: 6 }} flexDir="column">
            { blockQuery.isDegradedData && <ServiceDegradationWarning isLoading={ blockQuery.isPlaceholderData }/> }
            { blockQuery.data?.is_pending_update && <BlockPendingUpdateAlert/> }
          </Flex>
          <BlockDetails query={ blockQuery }/>
        </>
      ),
    },
    {
      id: 'txs',
      title: 'Transactions',
      component: (
        <>
          { blockTxsQuery.isDegradedData && <ServiceDegradationWarning isLoading={ blockTxsQuery.isPlaceholderData } mb={{ base: 3, lg: 6 }}/> }
          <TxsWithFrontendSorting query={ blockTxsQuery } showBlockInfo={ false }/>
        </>
      ),
    },
    {
      id: 'internal_txs',
      title: 'Internal txns',
      component: (
        <>
          { blockTxsQuery.isDegradedData && <ServiceDegradationWarning isLoading={ blockTxsQuery.isPlaceholderData } mb={{ base: 3, lg: 6 }}/> }
          <BlockInternalTxs query={ blockInternalTxsQuery } itemsCount={ blockQuery.data?.internal_transactions_count }/>
        </>
      ),
    },
    config.features.dataAvailability.isEnabled && blockQuery.data?.blob_transactions_count ?
      {
        id: 'blob_txs',
        title: 'Blob txns',
        component: (
          <TxsWithFrontendSorting query={ blockBlobTxsQuery } showBlockInfo={ false }/>
        ),
      } : null,
    beaconChainFeature.isEnabled && !beaconChainFeature.withdrawalsOnly && Boolean(blockQuery.data?.beacon_deposits_count) ?
      {
        id: 'deposits',
        title: 'Deposits',
        component: (
          <>
            { blockDepositsQuery.isDegradedData && <ServiceDegradationWarning isLoading={ blockDepositsQuery.isPlaceholderData } mb={{ base: 3, lg: 6 }}/> }
            <BlockDeposits blockDepositsQuery={ blockDepositsQuery } itemsCount={ blockQuery.data?.beacon_deposits_count }/>
          </>
        ),
      } : null,
    config.features.beaconChain.isEnabled && Boolean(blockQuery.data?.withdrawals_count) ?
      {
        id: 'withdrawals',
        title: 'Withdrawals',
        component: (
          <>
            { blockWithdrawalsQuery.isDegradedData &&
              <ServiceDegradationWarning isLoading={ blockWithdrawalsQuery.isPlaceholderData } mb={{ base: 3, lg: 6 }}/> }
            <BlockWithdrawals blockWithdrawalsQuery={ blockWithdrawalsQuery } itemsCount={ blockQuery.data?.withdrawals_count }/>
          </>
        ),
      } : null,
  ].filter(Boolean) as Array<BlockTab>), [
    blockBlobTxsQuery, blockDepositsQuery, blockInternalTxsQuery, blockQuery, blockTxsQuery, blockWithdrawalsQuery,
  ]);

  const transactionTabs = tabs.filter(({ id }) => [ 'txs', 'internal_txs', 'blob_txs', 'deposits' ].includes(String(id)));
  const entityTabs = [
    entityTabGroup('txs', 'Transactions', transactionTabs),
    ...tabs.filter(({ id }) => id === 'withdrawals'),
    ...tabs.filter(({ id }) => id === 'index'),
  ];

  throwOnAbsentParamError(heightOrHash);

  if (blockQuery.isError) {
    if (!blockQuery.isDegradedData && blockQuery.error.status === 404 && !heightOrHash.startsWith('0x') && blockQuery.isFutureBlock) {
      const url = routeParams({ pathname: '/block/countdown/[height]', query: { height: heightOrHash } }, multichainContext);
      router.push(url, undefined, { shallow: true });
      return null;
    } else {
      throwOnResourceLoadError(blockQuery);
    }
  }

  const titleText = (() => {
    switch (blockQuery.data?.type) {
      case 'reorg':
        return 'Reorged block';

      case 'uncle':
        return 'Uncle block';

      default:
        return 'Block';
    }
  })();

  const beforeTitleElement = multichainContext?.chain ? (
    <BlockEntity.Icon variant="heading" chain={ multichainContext.chain } isLoading={ blockQuery.isPlaceholderData }/>
  ) : null;

  const titleSecondRow = (
    <>
      { !config.UI.views.block.hiddenFields?.miner && blockQuery.data?.miner && (
        <Skeleton
          loading={ blockQuery.isPlaceholderData }
          fontFamily="heading"
          display="flex"
          minW={ 0 }
          columnGap={ 2 }
          fontWeight={ 500 }
        >
          <chakra.span flexShrink={ 0 }>
            { capitalize(getNetworkValidatorTitle()) }
          </chakra.span>
          <AddressEntity address={ blockQuery.data.miner }/>
        </Skeleton>
      ) }
      <NetworkExplorers
        type="block"
        pathParam={ heightOrHash }
        ml={{ base: config.UI.views.block.hiddenFields?.miner ? 0 : 3, lg: 'auto' }}
      />
    </>
  );

  const apiEntry = apiDocsFeature.isEnabled ? (
    <Link href={ route({ pathname: '/api-docs' }, multichainContext) } textStyle="sm" display="inline-flex" alignItems="center" data-api-entry>
      <IconSvg name="API" boxSize={ 4 } mr={ 1 }/>
      API
    </Link>
  ) : null;

  const finality = (() => {
    if (blockQuery.data?.is_pending_update) {
      return { label: 'Pending', bg: 'bg.surface', color: 'text.muted' };
    }
    if (blockQuery.data?.type === 'block') {
      return { label: 'Confirmed', bg: 'positiveSoft', color: 'positive' };
    }
    return { label: blockQuery.data?.type, bg: 'negativeSoft', color: 'negative' };
  })();

  return (
    <>
      <TextAd mb={ 6 }/>
      <PageTitle
        kind="BLOCK"
        title={ `${ titleText } #${ blockQuery.data?.height ?? heightOrHash }` }
        identifier={ blockQuery.data?.hash ?? heightOrHash }
        tags={ (
          <>
            <Badge borderRadius="full" loading={ blockQuery.isPlaceholderData } bg={ finality.bg } color={ finality.color }>
              { finality.label }
            </Badge>
            <BlockCeloEpochTag blockQuery={ blockQuery }/>
          </>
        ) }
        beforeTitle={ beforeTitleElement }
        secondRow={ titleSecondRow }
        isLoading={ blockQuery.isPlaceholderData }
      />
      <RoutedTabs tabs={ entityTabs } displayMode="entity" defaultTabId="txs" rightSlot={ apiEntry }/>
    </>
  );
};

export default BlockPageContent;
