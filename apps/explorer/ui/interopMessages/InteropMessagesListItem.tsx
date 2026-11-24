import { Flex, HStack, Text, Grid } from '@chakra-ui/react';
import React from 'react';

import type { InteropMessage } from 'types/api/interop';

import { spacing } from 'toolkit/theme/foundations/spacing';
import AddressFromToIcon from 'ui/shared/address/AddressFromToIcon';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import AddressEntityInterop from 'ui/shared/entities/address/AddressEntityInterop';
import ListItemMobile from 'ui/shared/ListItemMobile/ListItemMobile';
import InteropMessageStatus from 'ui/shared/statusTag/InteropMessageStatus';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

import InteropMessageAdditionalInfo from './InteropMessageAdditionalInfo';
import InteropMessageDestinationTx from './InteropMessageDestinationTx';
import InteropMessageSourceTx from './InteropMessageSourceTx';

interface Props {
  item: InteropMessage;
  isLoading?: boolean;
}

const InteropMessagesListItem = ({ item, isLoading }: Props) => {
  return (
    <ListItemMobile fontSize="small" lineHeight="small" rowGap={ 2 } containerType="inline-size">
      <Flex alignItems="center" justifyContent="space-between" w="100%">
        <InteropMessageStatus status={ item.status } isLoading={ isLoading }/>
        <InteropMessageAdditionalInfo payload={ item.payload } isLoading={ isLoading }/>
      </Flex>
      <Flex alignItems="flex-start" flexDirection="column" gap={ 2 } w="100%">
        <HStack w="100%" flexWrap="wrap">
          <Text fontWeight="medium" flexGrow={ 1 }>#{ item.nonce }</Text>
          <TimeWithTooltip timestamp={ item.timestamp } isLoading={ isLoading } color="text.secondary"/>
        </HStack>
        <Grid
          w="full"
          minW={ 0 }
          templateColumns="minmax(0, 1fr)"
          rowGap={ 2 }
          css={{ [ `@container (min-width: ${ spacing['80'].value })` ]: { gridTemplateColumns: 'var(--chakra-spacing-30) minmax(0, 1fr)' } }}
        >
          <Text as="span" color="text.secondary">Source tx</Text>
          <InteropMessageSourceTx { ...item } isLoading={ isLoading } truncation="tail"/>
          <Text as="span" color="text.secondary">Destination tx</Text>
          <InteropMessageDestinationTx { ...item } isLoading={ isLoading } truncation="tail"/>
        </Grid>
        <Flex gap={ 2 } flexWrap="wrap" minW={ 0 } w="full" justifyContent="space-between" mt={ 2 }>
          { item.init_chain !== undefined ? (
            <AddressEntityInterop
              chain={ item.init_chain }
              address={{ hash: item.sender_address_hash }}
              isLoading={ isLoading }
              truncation="tail"
              flex="1 1 var(--chakra-spacing-24)"
              minW={ 0 }
            />
          ) : (
            <AddressEntity
              address={{ hash: item.sender_address_hash }}
              isLoading={ isLoading }
              truncation="tail"
              flex="1 1 var(--chakra-spacing-24)"
              minW={ 0 }
            />
          ) }
          <AddressFromToIcon
            isLoading={ isLoading }
            type={ item.init_chain !== undefined ? 'in' : 'out' }
          />
          { item.relay_chain !== undefined ? (
            <AddressEntityInterop
              chain={ item.relay_chain }
              address={{ hash: item.target_address_hash }}
              isLoading={ isLoading }
              truncation="tail"
              flex="1 1 var(--chakra-spacing-24)"
              minW={ 0 }
            />
          ) : (
            <AddressEntity
              address={{ hash: item.target_address_hash }}
              isLoading={ isLoading }
              truncation="tail"
              flex="1 1 var(--chakra-spacing-24)"
              minW={ 0 }
            />
          ) }
        </Flex>
      </Flex>
    </ListItemMobile>
  );
};

export default React.memo(InteropMessagesListItem);
