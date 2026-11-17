import { Grid, VStack, Text } from '@chakra-ui/react';
import React from 'react';

import type { CctxListItem } from '@blockscout/zetachain-cctx-types';

import dayjs from 'lib/date/dayjs';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { spacing } from 'toolkit/theme/foundations/spacing';
import { SECOND } from 'toolkit/utils/consts';
import AddressEntityZetaChain from 'ui/shared/entities/address/AddressEntityZetaChain';
import TxEntityZetaChainCC from 'ui/shared/entities/tx/TxEntityZetaChainCC';
import TextSeparator from 'ui/shared/TextSeparator';
import Time from 'ui/shared/time/Time';
import ZetaChainCCTXReducedStatus from 'ui/shared/zetaChain/ZetaChainCCTXReducedStatus';
import ZetaChainCCTXValue from 'ui/shared/zetaChain/ZetaChainCCTXValue';

type Props = {
  tx: CctxListItem;
  isLoading?: boolean;
  animation?: string;
};

const LatestZetaChainCCTXItem = ({ tx, isLoading, animation }: Props) => {
  return (
    <VStack
      width="100%"
      minW={ 0 }
      containerType="inline-size"
      borderBottom="var(--chakra-border-widths-1) solid"
      borderColor="border.divider"
      py={ 3 }
      fontSize="sm"
      alignItems="start"
      gap={ 2 }
      animation={ animation }
    >
      <ZetaChainCCTXReducedStatus status={ tx.status_reduced } isLoading={ isLoading } type="full"/>
      <TxEntityZetaChainCC hash={ tx.index } isLoading={ isLoading } truncation="tail" w="full" fontWeight="semibold"/>
      <Skeleton loading={ isLoading } display="flex" flexWrap="wrap" color="text.secondary" gap={ 2 } justifyContent="start">
        { dayjs(Number(tx.last_update_timestamp) * SECOND).fromNow() }
        <TextSeparator mx={ 0 }/>
        <Time timestamp={ Number(tx.last_update_timestamp) * SECOND } format="lll_s"/>
      </Skeleton>
      <Grid
        w="full"
        minW={ 0 }
        gridTemplateColumns="minmax(0, 1fr)"
        gap={ 2 }
        css={{
          [ `@container (min-width: ${ spacing['80'].value })` ]: {
            gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 25) minmax(0, 1fr)',
          },
        }}
      >
        <Text>Sender</Text>
        <AddressEntityZetaChain
          address={{ hash: tx.sender_address }}
          chainId={ tx.source_chain_id.toString() }
          isLoading={ isLoading }
          truncation="constant"
        />
        <Text>Receiver</Text>
        <AddressEntityZetaChain
          address={{ hash: tx.receiver_address }}
          chainId={ tx.target_chain_id.toString() }
          isLoading={ isLoading }
          truncation="constant"
        />
        <Text>Asset</Text>
        <ZetaChainCCTXValue
          coinType={ tx.coin_type }
          tokenSymbol={ tx.token_symbol }
          amount={ tx.amount }
          decimals={ tx.decimals }
          isLoading={ isLoading }
        />
      </Grid>
    </VStack>
  );
};

export default React.memo(LatestZetaChainCCTXItem);
