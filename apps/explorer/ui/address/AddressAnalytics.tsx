import { Grid } from '@chakra-ui/react';
import React from 'react';

import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import StatTile from 'ui/shared/Page/StatTile';

import AddressCounterItem from './details/AddressCounterItem';
import type { AddressCountersQuery } from './utils/useAddressCountersQuery';
import type { AddressQuery } from './utils/useAddressQuery';

interface Props {
  addressQuery: AddressQuery;
  countersQuery: AddressCountersQuery;
}

const AddressAnalytics = ({ addressQuery, countersQuery }: Props) => (
  <DetailedInfo.Section label="ANALYTICS">
    <Grid templateColumns={{ base: 'minmax(0, 1fr)', md: 'repeat(3, minmax(0, 1fr))' }} gap={ 4 }>
      { ([
        [ 'transactions_count', 'Transactions', 'Number of transactions related to this address' ],
        [ 'token_transfers_count', 'Transfers', 'Number of transfers to/from this address' ],
        [ 'gas_usage_count', 'Gas used', 'Gas used by the address' ],
      ] as const).map(([ prop, label, hint ]) => (
        <StatTile key={ prop } label={ label } hint={ hint } isLoading={ addressQuery.isPlaceholderData || countersQuery.isPlaceholderData }>
          { countersQuery.data?.[prop] === undefined || countersQuery.data[prop] === null || countersQuery.isError ? '—' : (
            <AddressCounterItem
              prop={ prop }
              query={ countersQuery }
              address={ addressQuery.data?.hash ?? '' }
              isAddressQueryLoading={ addressQuery.isPlaceholderData }
              isDegradedData={ addressQuery.isDegradedData }
            />
          ) }
        </StatTile>
      )) }
    </Grid>
  </DetailedInfo.Section>
);

export default AddressAnalytics;
