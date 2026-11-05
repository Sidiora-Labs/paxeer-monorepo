import { Box, Center, Flex, HStack, Text } from '@chakra-ui/react';
import React from 'react';

import type { PriceUpdateItem } from 'types/api/paxeerXFeed';
import type { Transaction } from 'types/api/transaction';

import { route } from 'nextjs/routes';

import config from 'configs/app';
import { Badge } from 'toolkit/chakra/badge';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tag } from 'toolkit/chakra/tag';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import TxEntity from 'ui/shared/entities/tx/TxEntity';
import EntityTag from 'ui/shared/EntityTags/EntityTag';
import IconSvg from 'ui/shared/IconSvg';
import ScanMethodChip from 'ui/shared/scan/ScanMethodChip';
import TxStatus from 'ui/shared/statusTag/TxStatus';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';
import TxFee from 'ui/shared/tx/TxFee';
import TxWatchListTags from 'ui/shared/tx/TxWatchListTags';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';
import TxAdditionalInfo from 'ui/txs/TxAdditionalInfo';
import { isPriceUpdate, priceUpdateHref, priceUpdateMethod } from 'ui/txs/TxPriceUpdate';
import TxType from 'ui/txs/TxType';

type Props = {
  tx: Transaction | PriceUpdateItem;
  isLoading?: boolean;
};

export const isPriceUpdateItem = (tx: Transaction | PriceUpdateItem): tx is PriceUpdateItem => isPriceUpdate(tx);

export const PriceUpdateWatchListTags = ({ from, isLoading }: {
  from: React.ComponentProps<typeof AddressEntity>['address'];
  isLoading?: boolean;
}) => {
  const tags = (from.watchlist_names || []).filter(Boolean);

  if (tags.length === 0) {
    return null;
  }

  return (
    <Flex columnGap={ 2 } rowGap={ 2 } flexWrap="wrap" overflow="hidden" maxW="100%">
      { tags.map((tag) => (
        <Badge key={ tag.label } loading={ isLoading } truncated maxW={{ base: '115px', lg: '100%' }} colorPalette="gray">
          { tag.display_name }
        </Badge>
      )) }
    </Flex>
  );
};

const LatestTxsItem = ({ tx, isLoading }: Props) => {
  const priceUpdate = isPriceUpdateItem(tx);
  const evmTx = priceUpdate ? undefined : tx;
  const dataTo = evmTx?.to ? evmTx.to : evmTx?.created_contract;

  const protocolTag = evmTx?.to?.metadata?.tags?.find(tag => tag.tagType === 'protocol');

  return (
    <Flex
      data-latest-tx={ tx.hash }
      alignItems="center"
      columnGap={ 3 }
      px={ 4 }
      py={ 3 }
      borderBottomWidth="1px"
      borderStyle="solid"
      borderColor="border.divider"
    >
      <Center
        boxSize={ 9 }
        flexShrink={ 0 }
        borderRadius="md"
        borderWidth="1px"
        borderStyle="solid"
        borderColor="border.divider"
      >
        <IconSvg name="transactions" boxSize={ 5 } color="icon.secondary" isLoading={ isLoading }/>
      </Center>
      <Box minW={ 0 } w={{ lg: '150px', xl: '170px' }} flexShrink={ 0 }>
        <TxEntity
          isLoading={ isLoading }
          hash={ tx.hash }
          href={ priceUpdate ? priceUpdateHref(tx.hash) : undefined }
          noIcon
          textStyle="sm"
          fontWeight="500"
        />
        <Flex alignItems="center" flexWrap="wrap" columnGap={ 2 } rowGap={ 1 } mt="2px" minW={ 0 }>
          <TimeWithTooltip
            timestamp={ tx.timestamp }
            enableIncrement={ !isLoading }
            timeFormat="relative"
            isLoading={ isLoading }
            color="text.secondary"
            textStyle="xs"
            display="block"
          />
          <HStack gap={ 1 } minW={ 0 } data-label="tx-tags" _empty={{ display: 'none' }}>
            { priceUpdate ?
              <ScanMethodChip method={ priceUpdateMethod(tx) } isLoading={ isLoading }/> :
              <TxType types={ tx.transaction_types } isLoading={ isLoading }/> }
            { tx.status !== 'ok' && <TxStatus status={ tx.status } errorText={ tx.status === 'error' ? tx.result : undefined } isLoading={ isLoading }/> }
            { evmTx ?
              <TxWatchListTags tx={ evmTx } isLoading={ isLoading }/> :
              <PriceUpdateWatchListTags from={ tx.from } isLoading={ isLoading }/> }
            { protocolTag && <EntityTag data={ protocolTag } isLoading={ isLoading } minW="0" noColors/> }
          </HStack>
        </Flex>
      </Box>
      <Box minW={ 0 } flexGrow={ 1 } data-label="tx-parties">
        <Flex alignItems="center" columnGap={ 1 } minW={ 0 }>
          <Skeleton loading={ isLoading } textStyle="xs" color="text.muted" flexShrink={ 0 }>From</Skeleton>
          <AddressEntity address={ tx.from } isLoading={ isLoading } noIcon truncation="constant" textStyle="xs"/>
        </Flex>
        { (priceUpdate || dataTo) && (
          <Flex alignItems="center" columnGap={ 1 } minW={ 0 } mt="2px">
            <Skeleton loading={ isLoading } textStyle="xs" color="text.muted" flexShrink={ 0 }>To</Skeleton>
            { priceUpdate ? (
              <Link href={ route({ pathname: '/feed' }) } loading={ isLoading } textStyle="xs" whiteSpace="nowrap">Price feed</Link>
            ) : dataTo && <AddressEntity address={ dataTo } isLoading={ isLoading } noIcon truncation="constant" textStyle="xs"/> }
          </Flex>
        ) }
      </Box>
      { !(config.UI.views.tx.hiddenFields?.value && config.UI.views.tx.hiddenFields?.tx_fee) && (
        <Box flexShrink={ 0 } textAlign="right" data-label="tx-value">
          { !config.UI.views.tx.hiddenFields?.value && (
            <Tag variant="outlined" loading={ isLoading }>
              { evmTx ? <NativeCoinValue amount={ evmTx.value } accuracy={ 5 } loading={ isLoading }/> : '-' }
            </Tag>
          ) }
          { !config.UI.views.tx.hiddenFields?.tx_fee && (
            <Skeleton loading={ isLoading } display="flex" justifyContent="flex-end" whiteSpace="pre" textStyle="xs" color="text.muted" mt="2px">
              <Text as="span">Fee </Text>
              { evmTx ? <TxFee tx={ evmTx } accuracy={ 5 } noUsd/> : '-' }
            </Skeleton>
          ) }
        </Box>
      ) }
      { evmTx && <TxAdditionalInfo tx={ evmTx } isLoading={ isLoading } flexShrink={ 0 }/> }
    </Flex>
  );
};

export default React.memo(LatestTxsItem);
