import { Box } from '@chakra-ui/react';
import { useQueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/router';
import React, { useEffect } from 'react';

import type { SocketMessage } from 'lib/socket/types';
import type { TabItemRegular } from 'toolkit/components/AdaptiveTabs/types';
import type { TokenInfo } from 'types/api/token';

import config from 'configs/app';
import useApiQuery, { getResourceKey } from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import * as metadata from 'lib/metadata';
import getQueryParamString from 'lib/router/getQueryParamString';
import useEtherscanRedirects from 'lib/router/useEtherscanRedirects';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';
import { NFT_TOKEN_TYPE_IDS } from 'lib/token/tokenTypes';
import * as addressStubs from 'stubs/address';
import * as tokenStubs from 'stubs/token';
import { getTokenHoldersStub } from 'stubs/token';
import { generateListStub } from 'stubs/utils';
import RoutedTabs from 'toolkit/components/RoutedTabs/RoutedTabs';
import Address3rdPartyWidgets from 'ui/address/Address3rdPartyWidgets';
import useAddress3rdPartyWidgets from 'ui/address/address3rdPartyWidgets/useAddress3rdPartyWidgets';
import AddressContract from 'ui/address/AddressContract';
import { CONTRACT_TAB_IDS } from 'ui/address/contract/utils';
import TextAd from 'ui/shared/ad/TextAd';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';
import { entityTabGroup } from 'ui/shared/RoutedTabs/EntityTabs';
import TokenHolders from 'ui/token/TokenHolders/TokenHolders';
import TokenInventory from 'ui/token/TokenInventory';
import TokenMarkets from 'ui/token/TokenMarkets';
import TokenMetadata from 'ui/token/TokenMetadata';
import TokenPageTitle from 'ui/token/TokenPageTitle';
import TokenTransfer from 'ui/token/TokenTransfer/TokenTransfer';
import useTokenQuery from 'ui/token/useTokenQuery';

export type TokenTabs = 'markets' | 'token_transfers' | 'holders' | 'metadata' | 'inventory';

const TokenPageContent = () => {
  const [ isQueryEnabled, setIsQueryEnabled ] = React.useState(false);
  const [ totalSupplySocket, setTotalSupplySocket ] = React.useState<{
    hash: string;
    chainId: string | undefined;
    totalSupply: string;
  }>();
  const router = useRouter();

  const scrollRef = React.useRef<HTMLDivElement>(null);

  const hashString = getQueryParamString(router.query.hash);
  const tab = getQueryParamString(router.query.tab);
  const ownerFilter = getQueryParamString(router.query.holder_address_hash) || undefined;

  useEtherscanRedirects();
  const queryClient = useQueryClient();
  const chainId = useMultichainContext()?.chain.id;
  const tokenQueryKey = React.useMemo(() => getResourceKey('general:token', {
    pathParams: { hash: hashString },
    chainId,
  }), [ hashString, chainId ]);

  const tokenQuery = useTokenQuery(hashString);

  // The address behind the token is asked for with the token itself: it feeds the contract tab and the
  // socket that used to gate it only reports a changing total supply.
  const addressQuery = useApiQuery('general:address', {
    pathParams: { hash: hashString },
    queryOptions: {
      enabled: Boolean(router.query.hash),
      placeholderData: addressStubs.ADDRESS_INFO,
    },
  });

  const tokenCountersQuery = useApiQuery('general:token_counters', {
    pathParams: { hash: hashString },
    queryOptions: {
      enabled: Boolean(router.query.hash),
      placeholderData: tokenStubs.TOKEN_COUNTERS,
    },
  });

  React.useEffect(() => {
    if (!totalSupplySocket) {
      return;
    }
    if (totalSupplySocket.hash !== hashString || totalSupplySocket.chainId !== chainId) {
      setTotalSupplySocket(undefined);
      return;
    }
    if (tokenQuery.data && !tokenQuery.isPlaceholderData) {
      queryClient.setQueryData(tokenQueryKey, (prevData: TokenInfo | undefined) => {
        if (prevData) {
          return { ...prevData, total_supply: totalSupplySocket.totalSupply };
        }
      });
      setTotalSupplySocket(undefined);
    }
  }, [ tokenQuery.data, tokenQuery.isPlaceholderData, totalSupplySocket, hashString, chainId, tokenQueryKey, queryClient ]);

  const handleTotalSupplyMessage: SocketMessage.TokenTotalSupply['handler'] = React.useCallback((payload) => {
    const prevData = queryClient.getQueryData<TokenInfo>(tokenQueryKey);
    if (!prevData) {
      setTotalSupplySocket({ hash: hashString, chainId, totalSupply: payload.total_supply });
    }
    queryClient.setQueryData(tokenQueryKey, (prevData: TokenInfo | undefined) => {
      if (prevData) {
        return { ...prevData, total_supply: payload.total_supply };
      }
    });
  }, [ queryClient, tokenQueryKey, hashString, chainId ]);

  const enableQuery = React.useCallback(() => setIsQueryEnabled(true), []);

  const channel = useSocketChannel({
    topic: `tokens:${ hashString?.toLowerCase() }`,
    isDisabled: !hashString,
    onJoin: enableQuery,
    onSocketError: enableQuery,
  });
  useSocketMessage({
    channel,
    event: 'total_supply',
    handler: handleTotalSupplyMessage,
  });

  const verifiedInfoQuery = useApiQuery('contractInfo:token_verified_info', {
    pathParams: { hash: tokenQuery.data?.address_hash, chainId: config.chain.id },
    queryOptions: { enabled: Boolean(tokenQuery.data) && !tokenQuery.isPlaceholderData && config.features.verifiedTokens.isEnabled },
  });

  useEffect(() => {
    // even if config.meta.seo.enhancedDataEnabled is enabled, we don't fetch contract info for the project description
    // so we need to update the metadata anyway.
    if (tokenQuery.data && !tokenQuery.isPlaceholderData && !verifiedInfoQuery.isPlaceholderData) {
      const apiData = {
        ...tokenQuery.data,
        symbol_or_name: tokenQuery.data.symbol ?? tokenQuery.data.name ?? '',
        description: verifiedInfoQuery.data?.projectDescription,
        projectName: verifiedInfoQuery.data?.projectName,
      };
      metadata.update({ pathname: '/token/[hash]', query: { hash: tokenQuery.data.address_hash } }, apiData);
    }
  }, [ tokenQuery.data, tokenQuery.isPlaceholderData, verifiedInfoQuery.isPlaceholderData, verifiedInfoQuery.data ]);

  // The lists below read the token, not the address, so they leave as soon as the token has answered.
  const hasData = Boolean(tokenQuery.data && !tokenQuery.isPlaceholderData);
  const hasInventoryTab = tokenQuery.data?.type && NFT_TOKEN_TYPE_IDS.includes(tokenQuery.data.type);

  const transfersQuery = useQueryWithPages({
    resourceName: 'general:token_transfers',
    pathParams: { hash: hashString },
    scrollRef,
    options: {
      enabled: Boolean(hasData && hashString && tab === 'token_transfers'),
      placeholderData: tokenStubs.getTokenTransfersStub(tokenQuery.data?.type),
    },
  });

  const inventoryQuery = useQueryWithPages({
    resourceName: 'general:token_inventory',
    pathParams: { hash: hashString },
    filters: ownerFilter ? { holder_address_hash: ownerFilter } : {},
    scrollRef,
    options: {
      enabled: Boolean(hasData && hashString && tab === 'inventory'),
      placeholderData: generateListStub<'general:token_inventory'>(tokenStubs.TOKEN_INSTANCE, 50, { next_page_params: { unique_token: 1 } }),
    },
  });

  const holdersQuery = useQueryWithPages({
    resourceName: 'general:token_holders',
    pathParams: { hash: hashString },
    scrollRef,
    options: {
      enabled: Boolean(hashString && tab === 'holders' && hasData),
      placeholderData: getTokenHoldersStub(tokenQuery.data?.type, null),
    },
  });

  const address3rdPartyWidgets = useAddress3rdPartyWidgets('token', false, isQueryEnabled);

  const isLoading =
    tokenQuery.isPlaceholderData ||
    addressQuery.isPlaceholderData ||
    (address3rdPartyWidgets.isEnabled && address3rdPartyWidgets.configQuery.isPlaceholderData);

  const counters = tokenCountersQuery.isPlaceholderData ? undefined : tokenCountersQuery.data;
  const transfersCount = counters?.transfers_count !== undefined ? Number(counters.transfers_count) : undefined;
  const holdersCount = counters?.token_holders_count !== undefined ? Number(counters.token_holders_count) : undefined;

  const tabs: Array<TabItemRegular> = [
    hasInventoryTab ? {
      id: 'inventory',
      title: 'Inventory',
      component: (
        <TokenInventory
          inventoryQuery={ inventoryQuery }
          tokenQuery={ tokenQuery }
          ownerFilter={ ownerFilter }
          shouldRender={ !isLoading }
        />
      ),
    } : undefined,
    {
      id: 'token_transfers',
      title: 'History',
      component: (
        <TokenTransfer
          transfersQuery={ transfersQuery }
          tokenQuery={ tokenQuery }
          shouldRender={ !isLoading }
          transfersCount={ transfersCount }
        />
      ),
    },
    {
      id: 'holders',
      title: 'Holders',
      component: (
        <TokenHolders
          token={ tokenQuery.data }
          holdersQuery={ holdersQuery }
          shouldRender={ !isLoading }
          holdersCount={ holdersCount }
        />
      ),
    },
    addressQuery.data?.is_contract ? {
      id: 'contract',
      title: 'Contract',
      component: <AddressContract addressData={ addressQuery.data } isLoading={ isLoading }/>,
      subTabs: CONTRACT_TAB_IDS,
    } : undefined,
    (address3rdPartyWidgets.isEnabled && address3rdPartyWidgets.items.length > 0) ? {
      id: 'widgets',
      title: 'Widgets',
      count: address3rdPartyWidgets.items.length,
      component: <Address3rdPartyWidgets shouldRender={ !isLoading } addressType="token" showAll/>,
    } : undefined,
  ].filter(Boolean);

  const entityTabs: Array<TabItemRegular> = [
    { id: 'markets', title: 'Markets', component: <TokenMarkets tokenQuery={ tokenQuery }/> },
    ...tabs.filter(({ id }) => id === 'token_transfers' || id === 'holders'),
    entityTabGroup('metadata', 'Metadata', [
      {
        id: 'metadata_info', title: 'Token info',
        component: <TokenMetadata tokenQuery={ tokenQuery } verifiedInfoQuery={ verifiedInfoQuery }/>,
      },
      ...tabs.filter(({ id }) => id === 'contract' || id === 'widgets'),
    ]),
    ...tabs.filter(({ id }) => id === 'inventory'),
  ];

  return (
    <>
      <TextAd mb={ 6 }/>

      <TokenPageTitle
        tokenQuery={ tokenQuery }
        addressQuery={ addressQuery }
        verifiedInfoQuery={ verifiedInfoQuery }
        hash={ hashString }
      />

      { /* should stay before tabs to scroll up with pagination */ }
      <Box ref={ scrollRef }></Box>

      <RoutedTabs
        tabs={ entityTabs }
        displayMode="entity"
        variant="pill"
        size="sm"
        isLoading={ isLoading }
      />
    </>
  );
};

export default TokenPageContent;
