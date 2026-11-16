import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { ArbitrumL2MessagesItem } from 'types/api/arbitrumL2';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { layerLabels } from 'lib/rollups/utils';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { spacing } from 'toolkit/theme/foundations/spacing';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import BlockEntityL1 from 'ui/shared/entities/block/BlockEntityL1';
import TxEntity from 'ui/shared/entities/tx/TxEntity';
import TxEntityL1 from 'ui/shared/entities/tx/TxEntityL1';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';
import ArbitrumL2MessageStatus from 'ui/shared/statusTag/ArbitrumL2MessageStatus';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

import type { MessagesDirection } from './ArbitrumL2Messages';

const rollupFeature = config.features.rollup;

type Props = { item: ArbitrumL2MessagesItem; isLoading?: boolean; direction: MessagesDirection };

const ArbitrumL2MessagesListItem = ({ item, isLoading, direction }: Props) => {
  if (!rollupFeature.isEnabled || rollupFeature.type !== 'arbitrum') {
    return null;
  }

  const l1TxHash = direction === 'from-rollup' ? item.completion_transaction_hash : item.origination_transaction_hash;
  const l2TxHash = direction === 'from-rollup' ? item.origination_transaction_hash : item.completion_transaction_hash;

  return (
    <Box containerType="inline-size" w="full" minW={ 0 }>
      <ListItemMobileGrid.Container
        gridTemplateColumns="minmax(0, 1fr)"
        css={{ [ `@container (min-width: ${ spacing['80'].value })` ]: { gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 27.5) minmax(0, 1fr)' } }}
      >

        { direction === 'to-rollup' && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.parent } block</ListItemMobileGrid.Label>
            <ListItemMobileGrid.Value>
              { item.origination_transaction_block_number ? (
                <BlockEntityL1
                  number={ item.origination_transaction_block_number }
                  isLoading={ isLoading }
                  fontWeight="semibold"
                />
              ) : <chakra.span>N/A</chakra.span> }
            </ListItemMobileGrid.Value>
          </>
        ) }

        { direction === 'from-rollup' && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>From</ListItemMobileGrid.Label>
            <ListItemMobileGrid.Value>
              <AddressEntity
                address={{ hash: item.origination_address_hash }}
                truncation="tail"
                isLoading={ isLoading }
                fontWeight="semibold"
              />
            </ListItemMobileGrid.Value>
          </>
        ) }

        <ListItemMobileGrid.Label isLoading={ isLoading }>Message #</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          <Skeleton loading={ isLoading } display="inline-block">
            { item.id }
          </Skeleton>
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.current } transaction</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          { l2TxHash ? (
            <TxEntity
              isLoading={ isLoading }
              hash={ l2TxHash }
              truncation="tail"
            />
          ) : (
            <chakra.span>
              N/A
            </chakra.span>
          ) }
        </ListItemMobileGrid.Value>

        { item.origination_timestamp && (
          <>
            <ListItemMobileGrid.Label isLoading={ isLoading }>Age</ListItemMobileGrid.Label>
            <ListItemMobileGrid.Value>
              <TimeWithTooltip
                timestamp={ item.origination_timestamp }
                isLoading={ isLoading }
                display="inline-block"
              />
            </ListItemMobileGrid.Value>
          </>
        ) }

        <ListItemMobileGrid.Label isLoading={ isLoading }>Status</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          { item.status === 'confirmed' && direction === 'from-rollup' ?
            <Link href={ route({ pathname: '/txn-withdrawals', query: { q: item.origination_transaction_hash } }) }>Ready for relay</Link> :
            <ArbitrumL2MessageStatus status={ item.status } isLoading={ isLoading }/> }
        </ListItemMobileGrid.Value>

        <ListItemMobileGrid.Label isLoading={ isLoading }>{ layerLabels.parent } transaction</ListItemMobileGrid.Label>
        <ListItemMobileGrid.Value>
          { l1TxHash ? (
            <TxEntityL1
              isLoading={ isLoading }
              hash={ l1TxHash }
              truncation="tail"
              noCopy
            />
          ) : (
            <chakra.span>
              N/A
            </chakra.span>
          ) }
        </ListItemMobileGrid.Value>

      </ListItemMobileGrid.Container>
    </Box>
  );
};

export default ArbitrumL2MessagesListItem;
