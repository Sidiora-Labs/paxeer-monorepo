import { Box } from '@chakra-ui/react';
import React from 'react';

import type { CeloEpochDetails } from 'types/api/epochs';

import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';

import EpochElectionRewardsListItem from './EpochElectionRewardsListItem';
import EpochElectionRewardsTableItem from './EpochElectionRewardsTableItem';

interface Props {
  data: CeloEpochDetails;
  isLoading?: boolean;
}

const EpochElectionRewards = ({ data, isLoading }: Props) => {
  if (!data.aggregated_election_rewards) {
    return null;
  }

  return (
    <DetailedInfo.Section label="Election rewards">
      <Box hideBelow="lg">
        <TableRoot style={{ tableLayout: 'auto' }}>
          <TableHeaderSticky bg="canvas">
            <TableRow>
              <TableColumnHeader width="24px"/>
              <TableColumnHeader width="180px">Reward type</TableColumnHeader>
              <TableColumnHeader/>
              <TableColumnHeader isNumeric>Value</TableColumnHeader>
            </TableRow>
          </TableHeaderSticky>
          <TableBody>
            { Object.entries(data.aggregated_election_rewards).map((entry) => {
              const key = entry[0] as keyof CeloEpochDetails['aggregated_election_rewards'];
              const value = entry[1];

              if (!value) {
                return null;
              }

              return (
                <EpochElectionRewardsTableItem
                  key={ key }
                  type={ key }
                  isLoading={ isLoading }
                  data={ value }
                />
              );
            }) }
          </TableBody>
        </TableRoot>
      </Box>
      <Box hideFrom="lg">
        { Object.entries(data.aggregated_election_rewards).map((entry) => {
          const key = entry[0] as keyof CeloEpochDetails['aggregated_election_rewards'];
          const value = entry[1];

          if (!value) {
            return null;
          }

          return (
            <EpochElectionRewardsListItem
              key={ key }
              type={ key }
              isLoading={ isLoading }
              data={ value }
            />
          );
        }) }
      </Box>
    </DetailedInfo.Section>
  );
};

export default React.memo(EpochElectionRewards);
