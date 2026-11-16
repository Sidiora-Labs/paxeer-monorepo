import { useRouter } from 'next/router';
import React from 'react';
import { flushSync } from 'react-dom';

import type { Props } from './types';

import * as Layout from './components';

const LayoutSearchResults = ({ children }: Props) => {
  const router = useRouter();
  const [ , refresh ] = React.useReducer((value: number) => value + 1, 0);
  const tab = Array.isArray(router.query.tab) ? router.query.tab[0] : router.query.tab;
  const content = React.isValidElement(children) ?
    React.cloneElement(children, { key: tab ?? 'all' }) :
    children;

  React.useEffect(() => {
    const handleRouteChange = () => {
      flushSync(refresh);
    };

    router.events.on('routeChangeComplete', handleRouteChange);
    return () => router.events.off('routeChangeComplete', handleRouteChange);
  }, [ router.events ]);

  const handleSearchSubmit = React.useCallback((event: React.FormEvent<HTMLDivElement>) => {
    const target = event.target;
    if (!(target instanceof HTMLFormElement)) {
      return;
    }

    const input = target.querySelector<HTMLInputElement>('input[aria-label="Search"]');
    if (!input || !input.value || input.value === router.query.q) {
      return;
    }

    void router.push(
      { pathname: router.pathname, query: { ...router.query, q: input.value } },
      undefined,
      { shallow: true },
    );
  }, [ router ]);

  return (
    <Layout.Root content={ content }>
      <Layout.Container onSubmit={ handleSearchSubmit }>{ content }</Layout.Container>
    </Layout.Root>
  );
};

export default LayoutSearchResults;
