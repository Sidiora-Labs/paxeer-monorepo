import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { CeloEpochElectionReward, CeloEpochDetails } from 'types/api/epochs';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import EpochRewardTypeTag from 'ui/shared/EpochRewardTypeTag';
import IconSvg from 'ui/shared/IconSvg';
import TokenValue from 'ui/shared/value/TokenValue';

import EpochElectionRewardDetailsMobile from './EpochElectionRewardDetailsMobile';
import { getRewardNumText } from './utils';

interface Props {
  data: CeloEpochElectionReward;
  type: keyof CeloEpochDetails['aggregated_election_rewards'];
  isLoading?: boolean;
}

const EpochElectionRewardsListItem = ({ data, isLoading, type }: Props) => {
  const section = useDisclosure();
  const detailsId = React.useId();

  return (
    <Box
      py={ 3 }
      borderBottomWidth="1"
      borderColor="border.divider"
      fontSize="sm"
    >
      <Flex my="var(--chakra-spacing-0\.5)" columnGap={ 3 } alignItems="center" flexWrap="wrap" rowGap={ 2 }
      >
        { data.count ? (
          <Skeleton loading={ isLoading } display="flex" borderRadius="sm">
            <IconButton
              aria-label={ section.open ? 'Collapse section' : 'Expand section' }
              onClick={ section.onToggle }
              aria-expanded={ section.open }
              aria-controls={ detailsId }
              disabled={ isLoading }
              variant="link"
              minH="touchTarget"
              minW="touchTarget"
            >
              <IconSvg
                name="arrows/east-mini"
                boxSize={ 6 }
                transform={ section.open ? 'rotate(270deg)' : 'rotate(180deg)' }
                _motionSafe={{ transitionProperty: 'transform', transitionDuration: 'faster' }}
              />
            </IconButton>
          </Skeleton>
        ) : <Box boxSize={ 6 }/> }
        <EpochRewardTypeTag type={ type } isLoading={ isLoading }/>
        <Skeleton loading={ isLoading } ml="auto">{ getRewardNumText(type, data.count) }</Skeleton>
        <TokenValue
          amount={ data.total }
          token={ data.token }
          accuracy={ 0 }
          loading={ isLoading }
          fontWeight="medium"
        />
      </Flex>
      { section.open && (
        <Box id={ detailsId } mt={ 2 }>
          <EpochElectionRewardDetailsMobile type={ type } token={ data.token }/>
        </Box>
      ) }
    </Box>
  );
};

export default React.memo(EpochElectionRewardsListItem);
