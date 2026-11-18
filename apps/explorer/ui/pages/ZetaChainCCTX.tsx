import { useRouter } from 'next/router';
import React from 'react';

import type { CrossChainTx } from '@blockscout/zetachain-cctx-types';

import type { ResourceError } from 'lib/api/resources';
import useApiQuery from 'lib/api/useApiQuery';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import getQueryParamString from 'lib/router/getQueryParamString';
import { ZETA_CHAIN_CCTX } from 'stubs/zetaChainCCTX';
import TextAd from 'ui/shared/ad/TextAd';
import TxEntityZetaChainCC from 'ui/shared/entities/tx/TxEntityZetaChainCC';
import PageTitle from 'ui/shared/Page/PageTitle';
import ZetaChainCCTXDetails from 'ui/zetaChain/cctxDetails/ZetaChainCCTXDetails';

export interface ZetaChainCCTXProps {
  initialTransaction?: CrossChainTx | null;
  initialError?: ResourceError | null;
}

const ZetaChainCCTX = ({ initialTransaction, initialError }: ZetaChainCCTXProps) => {
  const router = useRouter();

  const hash = getQueryParamString(router.query.hash);

  const cctxQuery = useApiQuery('zetachain:transaction', {
    queryParams: { cctx_id: hash },
    queryOptions: {
      placeholderData: ZETA_CHAIN_CCTX,
      initialData: initialTransaction ?? undefined,
      enabled: !initialError,
    },
  });

  if (initialError) {
    throwOnResourceLoadError({ isError: true, error: initialError, resource: 'zetachain:transaction' });
  }
  throwOnResourceLoadError(cctxQuery);

  return (
    <>
      <TextAd mb={ 6 }/>
      <PageTitle
        title="Cross-chain tx details"
        secondRow={ <TxEntityZetaChainCC hash={ hash } noLink variant="subheading" mr={{ base: 0, lg: 2 }}/> }
      />
      <ZetaChainCCTXDetails data={ cctxQuery.data } isLoading={ cctxQuery.isPlaceholderData }/>
    </>
  );
};

export default ZetaChainCCTX;
