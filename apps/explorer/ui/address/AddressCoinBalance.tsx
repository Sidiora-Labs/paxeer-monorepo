import { Box } from '@chakra-ui/react';
import dynamic from 'next/dynamic';
import { useRouter } from 'next/router';
import React from 'react';

import useIsMounted from 'lib/hooks/useIsMounted';
import getQueryParamString from 'lib/router/getQueryParamString';
import { ADDRESS_COIN_BALANCE } from 'stubs/address';
import { generateListStub } from 'stubs/utils';
import { Skeleton } from 'toolkit/chakra/skeleton';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';
import { ScanTableCard } from 'ui/shared/scan';
import SocketAlert from 'ui/shared/SocketAlert';

import AddressCoinBalanceHistory from './coinBalance/AddressCoinBalanceHistory';
import useCoinBalanceReconciliation from './useCoinBalanceReconciliation';

// The chart draws itself with d3 in the browser and nowhere else, and it is the only reason this tab
// carries that code, so it arrives on its own. The placeholder holds the height of the widget - its
// 300px canvas inside the titled, padded frame - so the history beneath it does not move when the
// chart lands.
const CHART_HEIGHT = '366px';

const AddressCoinBalanceChart = dynamic(() => import('./coinBalance/AddressCoinBalanceChart'), {
  ssr: false,
  loading: () => (
    <Skeleton
      data-coin-balance-chart-skeleton
      loading
      w="100%"
      h={ CHART_HEIGHT }
      borderRadius="lg"
    />
  ),
});

type Props = {
  shouldRender?: boolean;
  isQueryEnabled?: boolean;
};

const AddressCoinBalance = ({ shouldRender = true, isQueryEnabled = true }: Props) => {
  const router = useRouter();
  const isMounted = useIsMounted();

  const scrollRef = React.useRef<HTMLDivElement>(null);

  const addressHash = getQueryParamString(router.query.hash);
  const coinBalanceQuery = useQueryWithPages({
    resourceName: 'general:address_coin_balance',
    pathParams: { hash: addressHash },
    scrollRef,
    options: {
      enabled: isQueryEnabled,
      placeholderData: generateListStub<'general:address_coin_balance'>(
        ADDRESS_COIN_BALANCE,
        50,
        {
          next_page_params: {
            block_number: 8009880,
            items_count: 50,
          },
        },
      ),
    },
  });

  const socketAlert = useCoinBalanceReconciliation(
    addressHash,
    isQueryEnabled && !coinBalanceQuery.isPlaceholderData && coinBalanceQuery.pagination.page === 1,
  );

  if (!isMounted || !shouldRender) {
    return null;
  }

  const itemsNum = coinBalanceQuery.data?.items.length ?? 0;

  return (
    <>
      { socketAlert && <SocketAlert mb={ 6 }/> }
      <Box
        data-coin-balance-chart-card
        bg="bg.surface"
        borderWidth="1px"
        borderStyle="solid"
        borderColor="border.divider"
        borderRadius="md"
        boxShadow="card"
        px={ 4 }
        py={ 4 }
        mb={ 6 }
      >
        <AddressCoinBalanceChart addressHash={ addressHash }/>
      </Box>
      <div ref={ scrollRef }></div>
      <ScanTableCard title={ `${ itemsNum.toLocaleString() } visible balance changes` }>
        <AddressCoinBalanceHistory query={ coinBalanceQuery }/>
      </ScanTableCard>
    </>
  );
};

export default React.memo(AddressCoinBalance);
