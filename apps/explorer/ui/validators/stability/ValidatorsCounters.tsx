import { Box } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { VALIDATORS_STABILITY_COUNTERS } from 'stubs/validators';
import { ScanStatCard } from 'ui/shared/scan';

const ValidatorsCounters = () => {
  const countersQuery = useApiQuery('general:validators_stability_counters', {
    queryOptions: {
      enabled: config.features.validators.isEnabled,
      placeholderData: VALIDATORS_STABILITY_COUNTERS,
    },
  });

  if (!countersQuery.data) {
    return null;
  }

  return (
    <Box columnGap={ 3 } rowGap={ 3 } mb={ 6 } display="grid" gridTemplateColumns={{ base: '1fr', lg: 'repeat(2, 1fr)' }}>
      <ScanStatCard
        label="Total validators"
        value={ Number(countersQuery.data.validators_count).toLocaleString() }
        secondary={ `${ Number(countersQuery.data.new_validators_count_24h).toLocaleString() } new in 24h` }
        isLoading={ countersQuery.isPlaceholderData }
      />
      <ScanStatCard
        label="Active validators"
        value={ Number(countersQuery.data.active_validators_count).toLocaleString() }
        secondary={ `${ Number(countersQuery.data.active_validators_percentage).toLocaleString() }%` }
        isLoading={ countersQuery.isPlaceholderData }
      />
    </Box>
  );
};

export default React.memo(ValidatorsCounters);
