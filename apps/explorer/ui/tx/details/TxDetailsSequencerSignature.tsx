import { Flex, Text } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import StatusLadderBadge from 'ui/shared/statusLadder/StatusLadderBadge';

const feature = config.features.paxeerXLists;

interface Props {
  hash: string | undefined;
}

export default function TxDetailsSequencerSignature({ hash }: Props) {
  const { data, isError } = useApiQuery('general:paxeer_x_tx_signature', {
    pathParams: { hash },
    queryOptions: {
      enabled: feature.isEnabled && Boolean(hash),
      refetchOnMount: false,
    },
  });

  if (!feature.isEnabled || !hash || isError || !data) {
    return null;
  }

  return (
    <>
      <DetailedInfo.ItemLabel>Sequencer signature</DetailedInfo.ItemLabel>
      <DetailedInfo.ItemValue>
        <Flex direction="column" alignItems="flex-start" gap={ 2 } minW={ 0 } whiteSpace="normal">
          <StatusLadderBadge rung={ data.rung }/>
          <Text wordBreak="break-all">Key id: { data.receipt_key.key_id }</Text>
          <Text fontFamily="mono" wordBreak="break-all">{ data.signature }</Text>
        </Flex>
      </DetailedInfo.ItemValue>
    </>
  );
}
