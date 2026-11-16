import { HStack } from '@chakra-ui/react';
import React from 'react';

import { route } from 'nextjs-routes';

import { Link } from 'toolkit/chakra/link';
import { Tag } from 'toolkit/chakra/tag';
import { Tooltip } from 'toolkit/chakra/tooltip';

import type { BlockQuery } from './useBlockQuery';

interface Props {
  blockQuery: BlockQuery;
}

const BlockCeloEpochTagRegular = ({ blockQuery }: Props) => {
  if (!blockQuery.data?.celo) {
    return null;
  }

  return (
    <Tooltip
      key="epoch-tag-before-finalized"
      content="Displays the epoch this block belongs to before the epoch is finalized"
    >
      <Link
        href={ route({ pathname: '/epochs/[number]', query: { number: String(blockQuery.data.celo.epoch_number) } }) }
        display="inline-flex"
        alignItems="center"
        minW="touchTarget"
        maxW="full"
        minH="touchTarget"
        _motionReduce={{ transitionProperty: 'none' }}
      >
        <Tag variant="clickable">Epoch #{ blockQuery.data.celo.epoch_number }</Tag>
      </Link>
    </Tooltip>
  );
};

const BlockCeloEpochTag = ({ blockQuery }: Props) => {
  if (!blockQuery.data?.celo) {
    return null;
  }

  if (!blockQuery.data.celo.l1_era_finalized_epoch_number) {
    return <BlockCeloEpochTagRegular blockQuery={ blockQuery }/>;
  }

  return (
    <HStack gap={ 2 } flexWrap="wrap" minW={ 0 } maxW="full">
      <Tooltip
        key="epoch-tag"
        content="Displays the epoch finalized by this block"
      >
        <Link
          href={ route({ pathname: '/epochs/[number]', query: { number: String(blockQuery.data.celo.l1_era_finalized_epoch_number) } }) }
          display="inline-flex"
          alignItems="center"
          minW="touchTarget"
          maxW="full"
          minH="touchTarget"
          _motionReduce={{ transitionProperty: 'none' }}
        >
          <Tag
            bgColor="positiveSoft"
            color="positive"
            variant="clickable"
            borderRadius="md"
            css={{ '& > .chakra-tag__label': { whiteSpace: 'normal', overflowWrap: 'anywhere', overflow: 'visible' } }}
          >
            Finalized epoch #{ blockQuery.data.celo.l1_era_finalized_epoch_number }
          </Tag>
        </Link>
      </Tooltip>
      <BlockCeloEpochTagRegular blockQuery={ blockQuery }/>
    </HStack>
  );
};

export default React.memo(BlockCeloEpochTag);
