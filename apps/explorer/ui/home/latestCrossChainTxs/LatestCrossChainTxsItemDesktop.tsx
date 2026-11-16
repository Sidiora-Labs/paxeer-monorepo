import { chakra, VStack } from '@chakra-ui/react';
import React from 'react';

import type { InterchainMessage } from '@blockscout/interchain-indexer-types';

import { TableCell, TableRow } from 'toolkit/chakra/table';
import { mdash } from 'toolkit/utils/htmlEntities';
import CrossChainBridgeLink from 'ui/shared/crossChain/CrossChainBridgeLink';
import CrossChainMessageEntity from 'ui/shared/entities/crossChainMessage/CrossChainMessageEntity';
import TxEntityInterchain from 'ui/shared/entities/tx/TxEntityInterchain';
import ChainLabel from 'ui/shared/externalChains/ChainLabel';
import CrossChainTxsStatusTag from 'ui/shared/statusTag/CrossChainTxsStatusTag';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

interface Props {
  data: InterchainMessage;
  isLoading?: boolean;
}

const LatestCrossChainTxsItemDesktop = ({ data, isLoading }: Props) => {

  return (
    <TableRow>
      <TableCell w="var(--chakra-spacing-10)">
        <CrossChainTxsStatusTag status={ data.status } loading={ isLoading }/>
      </TableCell>
      <TableCell>
        <VStack alignItems="start">
          <CrossChainMessageEntity id={ data.message_id } isLoading={ isLoading } lineHeight="h2" fontWeight="bold"/>
          <TimeWithTooltip timestamp={ data.send_timestamp || data.receive_timestamp } isLoading={ isLoading } color="text.secondary" timeFormat="absolute"/>
        </VStack>
      </TableCell>
      <TableCell>
        <VStack alignItems="start">
          { data.source_transaction_hash ? (
            <TxEntityInterchain
              chain={ data.source_chain }
              hash={ data.source_transaction_hash }
              isLoading={ isLoading }
              noIcon
              noCopy
              truncation="constant"
              lineHeight="h2"
            />
          ) : (
            <chakra.span color="text.secondary">{ mdash }</chakra.span>
          ) }
          <ChainLabel
            data={ data.source_chain }
            isLoading={ isLoading }
            color="text.secondary"
            textStyle="text.xs"
          />
        </VStack>
      </TableCell>
      <TableCell>
        <VStack alignItems="start">
          { data.destination_transaction_hash ? (
            <TxEntityInterchain
              chain={ data.destination_chain }
              hash={ data.destination_transaction_hash }
              isLoading={ isLoading }
              noIcon
              noCopy
              truncation="constant"
              lineHeight="h2"
            />
          ) : (
            <chakra.span color="text.secondary">{ mdash }</chakra.span>
          ) }
          <ChainLabel
            data={ data.destination_chain }
            isLoading={ isLoading }
            color="text.secondary"
            textStyle="text.xs"
          />
        </VStack>
      </TableCell>
      <TableCell>
        <CrossChainBridgeLink data={ data.bridge } isLoading={ isLoading } lineHeight="h2"/>
      </TableCell>
    </TableRow>
  );
};

export default React.memo(LatestCrossChainTxsItemDesktop);
