import { useRouter } from 'next/router';
import React from 'react';

import type { NFTTokenType } from 'types/api/token';

import { useAppContext } from 'lib/contexts/app';
import * as cookies from 'lib/cookies';
import getFilterValuesFromQuery from 'lib/getFilterValuesFromQuery';
import { NFT_TOKEN_TYPE_IDS } from 'lib/token/tokenTypes';
import { ADDRESS_COLLECTION, ADDRESS_NFT_1155 } from 'stubs/address';
import { generateListStub } from 'stubs/utils';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';

const getTokenFilterValue: (type: string | Array<string> | undefined) => Array<NFTTokenType> | undefined =
(getFilterValuesFromQuery<NFTTokenType>).bind(null, NFT_TOKEN_TYPE_IDS);

export type TNftDisplayType = 'collection' | 'list';

interface Props {
  scrollRef: React.RefObject<HTMLDivElement | null>;
  enabled?: boolean;
  addressHash: string;
  isMultichain?: boolean;
  chainIds?: Array<string>;
}

export default function useAddressNftQuery({ scrollRef, enabled = true, addressHash, isMultichain, chainIds }: Props) {
  const router = useRouter();

  const displayTypeCookie = cookies.get(cookies.NAMES.ADDRESS_NFT_DISPLAY_TYPE, useAppContext().cookies);
  const routeDisplayType = ((): TNftDisplayType | undefined => {
    if (router.query.tab === 'tokens_nfts_list') {
      return 'list';
    }
    if (router.query.tab === 'tokens_nfts_collection') {
      return 'collection';
    }
  })();
  const [ selectedDisplayType, setDisplayType ] = React.useState<TNftDisplayType>(displayTypeCookie === 'list' ? 'list' : 'collection');
  const displayType = routeDisplayType ?? selectedDisplayType;
  const [ tokenTypes, setTokenTypes ] = React.useState<Array<NFTTokenType> | undefined>(getTokenFilterValue(router.query.type) || []);

  const collectionsQuery = useQueryWithPages({
    resourceName: 'general:address_collections',
    pathParams: { hash: addressHash },
    scrollRef,
    options: {
      enabled: enabled && displayType === 'collection',
      placeholderData: generateListStub<'general:address_collections'>(ADDRESS_COLLECTION, 10, { next_page_params: null }),
    },
    filters: { type: tokenTypes },
    isMultichain,
    chainIds,
  });

  const nftsQuery = useQueryWithPages({
    resourceName: 'general:address_nfts',
    pathParams: { hash: addressHash },
    scrollRef,
    options: {
      enabled: enabled && displayType === 'list',
      placeholderData: generateListStub<'general:address_nfts'>(ADDRESS_NFT_1155, 10, { next_page_params: null }),
    },
    filters: { type: tokenTypes },
    isMultichain,
    chainIds,
  });

  const onDisplayTypeChange = React.useCallback((val: string) => {
    cookies.set(cookies.NAMES.ADDRESS_NFT_DISPLAY_TYPE, val);
    setDisplayType(val as TNftDisplayType);
    if (routeDisplayType) {
      router.push({ pathname: router.pathname, query: { ...router.query, tab: 'tokens_nfts' } }, undefined, { shallow: true });
    }
  }, [ routeDisplayType, router ]);

  const onTokenTypesChange = React.useCallback((value: Array<NFTTokenType>) => {
    nftsQuery.onFilterChange({ type: value });
    collectionsQuery.onFilterChange({ type: value });
    setTokenTypes(value);
  }, [ nftsQuery, collectionsQuery ]);

  return React.useMemo(() => ({
    nftsQuery,
    collectionsQuery,
    displayType,
    tokenTypes,
    onDisplayTypeChange,
    onTokenTypesChange,
  }), [ nftsQuery, collectionsQuery, displayType, tokenTypes, onDisplayTypeChange, onTokenTypesChange ]);
}
