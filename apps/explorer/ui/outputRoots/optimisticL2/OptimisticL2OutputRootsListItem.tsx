import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { OptimisticL2OutputRootsItem } from 'types/api/optimisticL2';

import config from 'configs/app';
import { layerLabels } from 'lib/rollups/utils';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
import { spacing } from 'toolkit/theme/foundations/spacing';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import BlockEntityL2 from 'ui/shared/entities/block/BlockEntityL2';
import TxEntityL1 from 'ui/shared/entities/tx/TxEntityL1';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

const rollupFeature = config.features.rollup;

type Props = { item: OptimisticL2OutputRootsItem; isLoading?: boolean };

const OptimisticL2OutputRootsListItem = ({ item, isLoading }: Props) => {
  if (!rollupFeature.isEnabled || rollupFeature.type !== 'optimistic') {
    return null;
  }

  return (
    <Box containerType="inline-size" w="full" minW={ 0 }>
      <ListItemMobileGrid.Container
        gridTemplateColumns="minmax(0, 1fr)"
        css={{ [ `@container (min-width: ${ spacing['80'].value })` ]: { gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 21.5) minmax(0, 1fr)' } }}
      >

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.current } output index</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value fontWeight="semibold" color="text.primary">
          <Skeleton loading={ isLoading } display="inline-block">{ item.l2_output_index }</Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Age</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <TimeWithTooltip
            timestamp={ item.l1_timestamp }
            isLoading={ isLoading }
            display="inline-block"
          />
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.current } block #</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <BlockEntityL2
            isLoading={ isLoading }
            number={ item.l2_block_number }
            noIcon
          />
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.parent } txn hash</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value py="calc(var(--chakra-spacing-1) * 0.75)">
          <TxEntityL1
            isLoading={ isLoading }
            hash={ item.l1_transaction_hash }
            truncation="tail"
            noCopy
          />
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Output root</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <Flex overflow="hidden" whiteSpace="nowrap" alignItems="center" w="100%" justifyContent="start">
            <Skeleton loading={ isLoading } color="text.secondary" minW={ 0 } flex={ 1 }>
              <TruncatedText text={ item.output_root } minW={ 0 } maxW="full"/>
            </Skeleton>
            <CopyToClipboard text={ item.output_root } isLoading={ isLoading }/>
          </Flex>
        </ListItemMobileGrid.Value>

      </ListItemMobileGrid.Container>
    </Box>
  );
};

export default OptimisticL2OutputRootsListItem;
