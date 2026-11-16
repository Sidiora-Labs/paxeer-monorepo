import { Box } from '@chakra-ui/react';
import React from 'react';

import type { OptimisticL2DisputeGamesItem } from 'types/api/optimisticL2';

import config from 'configs/app';
import { layerLabels } from 'lib/rollups/utils';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
import { spacing } from 'toolkit/theme/foundations/spacing';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import BlockEntityL2 from 'ui/shared/entities/block/BlockEntityL2';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

const rollupFeature = config.features.rollup;

type Props = { item: OptimisticL2DisputeGamesItem; isLoading?: boolean };

const OptimisticL2DisputeGamesListItem = ({ item, isLoading }: Props) => {
  if (!rollupFeature.isEnabled || rollupFeature.type !== 'optimistic') {
    return null;
  }

  return (
    <Box containerType="inline-size" w="full" minW={ 0 }>
      <ListItemMobileGrid.Container
        gridTemplateColumns="minmax(0, 1fr)"
        css={{ [ `@container (min-width: ${ spacing['80'].value })` ]: { gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 21.5) minmax(0, 1fr)' } }}
      >

        <ListItemMobileGrid.Label isLoading={ isLoading }>Index</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value fontWeight="semibold" color="text.primary">
          <Skeleton loading={ isLoading } display="inline-block">{ item.index }</Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Game type</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <Skeleton loading={ isLoading } display="inline-block">{ item.game_type }</Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Address</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value color="text.primary">
          <Skeleton loading={ isLoading } display="flex" overflow="hidden" w="100%" alignItems="center">
            <TruncatedText text={ item.contract_address_hash } minW={ 0 } maxW="full"/>
            <CopyToClipboard text={ item.contract_address_hash } ml={ 2 } isLoading={ isLoading }/>
          </Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.current } block #</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <BlockEntityL2
            isLoading={ isLoading }
            number={ item.l2_block_number }
            noIcon
          />
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Age</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <TimeWithTooltip
            timestamp={ item.created_at }
            isLoading={ isLoading }
            display="inline-block"
          />
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>Status</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value color="text.primary">
          <Skeleton loading={ isLoading } display="inline-block">{ item.status }</Skeleton>
        </ListItemMobileGrid.Value>

        { item.resolved_at && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>Resolution age</ListItemMobileGrid.Label><ListItemMobileGrid.Value>
              <TimeWithTooltip
                timestamp={ item.resolved_at }
                isLoading={ isLoading }
                display="inline-block"
              />
            </ListItemMobileGrid.Value>
          </>
        ) }

      </ListItemMobileGrid.Container>
    </Box>
  );
};

export default OptimisticL2DisputeGamesListItem;
