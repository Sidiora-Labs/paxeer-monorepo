import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import type { CrossChainTx } from '@blockscout/zetachain-cctx-types';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import { base } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';
import buildUrl from 'nextjs/utils/buildUrl';

import getQueryParamString from 'lib/router/getQueryParamString';
import type { ZetaChainCCTXProps } from 'ui/pages/ZetaChainCCTX';
import ZetaChainCCTX from 'ui/pages/ZetaChainCCTX';

type PageProps = Props & Required<ZetaChainCCTXProps>;

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <PageNextJs pathname="/cc/tx/[hash]" query={ props.query }>
      <ZetaChainCCTX initialTransaction={ props.initialTransaction } initialError={ props.initialError }/>
    </PageNextJs>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<PageProps> = async(context) => {
  const result = await base(context);
  if (!('props' in result)) {
    return result;
  }
  const props = await result.props;

  try {
    const url = buildUrl('zetachain:transaction', undefined, { cctx_id: getQueryParamString(context.query.hash) });
    const response = await fetch(url, { signal: AbortSignal.timeout(5_000) });
    if (!response.ok) {
      context.res.statusCode = response.status;
      return {
        props: { ...props, initialTransaction: null, initialError: { status: response.status, statusText: response.statusText } },
      };
    }
    const initialTransaction = await response.json() as CrossChainTx;
    return { props: { ...props, initialTransaction, initialError: null } };
  } catch (error) {
    const status = error instanceof Error && error.name === 'TimeoutError' ? 504 : 500;
    context.res.statusCode = status;
    return {
      props: { ...props, initialTransaction: null, initialError: { status, statusText: status === 504 ? 'Gateway Timeout' : 'Internal Server Error' } },
    };
  }
};
