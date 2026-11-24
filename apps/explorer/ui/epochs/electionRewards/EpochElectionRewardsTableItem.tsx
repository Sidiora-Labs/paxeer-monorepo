import React from 'react';

import type { CeloEpochDetails, CeloEpochElectionReward } from 'types/api/epochs';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import EpochRewardTypeTag from 'ui/shared/EpochRewardTypeTag';
import IconSvg from 'ui/shared/IconSvg';
import TokenValue from 'ui/shared/value/TokenValue';

import EpochElectionRewardDetailsDesktop from './EpochElectionRewardDetailsDesktop';
import { getRewardNumText } from './utils';

interface Props {
  data: CeloEpochElectionReward;
  type: keyof CeloEpochDetails['aggregated_election_rewards'];
  isLoading?: boolean;
}

const EpochElectionRewardsTableItem = ({ isLoading, data, type }: Props) => {
  const section = useDisclosure();
  const detailsId = React.useId();

  const mainRowBorderColor = section.open ? 'transparent' : 'border.divider';

  return (
    <>
      <TableRow

      >
        <TableCell borderColor={ mainRowBorderColor }>
          { Boolean(data.count) && (
            <Skeleton loading={ isLoading } display="flex" borderRadius="sm">
              <IconButton
                aria-label={ section.open ? 'Collapse section' : 'Expand section' }
                onClick={ section.onToggle }
                aria-expanded={ section.open }
                aria-controls={ detailsId }
                disabled={ isLoading }
                variant="link"
              >
                <IconSvg
                  name="arrows/east-mini"
                  boxSize={ 6 }
                  transform={ section.open ? 'rotate(270deg)' : 'rotate(180deg)' }
                  _motionSafe={{ transitionProperty: 'transform', transitionDuration: 'faster' }}
                />
              </IconButton>
            </Skeleton>
          ) }
        </TableCell>
        <TableCell borderColor={ mainRowBorderColor }>
          <EpochRewardTypeTag type={ type } isLoading={ isLoading }/>
        </TableCell>
        <TableCell borderColor={ mainRowBorderColor }>
          <Skeleton loading={ isLoading } fontWeight="normal" my={ 1 }>
            { getRewardNumText(type, data.count) }
          </Skeleton>
        </TableCell>
        <TableCell borderColor={ mainRowBorderColor } textAlign="right">
          <TokenValue
            amount={ data.total }
            token={ data.token }
            accuracy={ 0 }
            loading={ isLoading }
            my="var(--chakra-spacing-0\.5)"
          />
        </TableCell>
      </TableRow>
      { section.open && (
        <TableRow>
          <TableCell/>
          <TableCell id={ detailsId } colSpan={ 3 } pr={ 0 } pt={ 0 }>
            <EpochElectionRewardDetailsDesktop type={ type } token={ data.token }/>
          </TableCell>
        </TableRow>
      ) }
    </>
  );
};

export default React.memo(EpochElectionRewardsTableItem);
