import { Box } from '@chakra-ui/react';
import { createGraphiQLFetcher } from '@graphiql/toolkit';
import { GraphiQL } from 'graphiql';
import React from 'react';

import config from 'configs/app';
import buildUrl from 'lib/api/buildUrl';
import 'graphiql/graphiql.css';
import useApiQuery from 'lib/api/useApiQuery';
import { useColorMode } from 'toolkit/chakra/color-mode';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import { ZERO_ADDRESS } from 'toolkit/utils/consts';

const feature = config.features.apiDocs;

const graphQLStyle = {
  '.graphiql-container': {
    backgroundColor: 'unset',
  },
  '.graphiql-logo-link, .cm-s-graphiql .cm-attribute': {
    color: 'text.secondary',
  },
  '.cm-s-graphiql .cm-property': {
    color: 'link.primary',
  },
};

const GraphQL = () => {

  const latestTxsQuery = useApiQuery('general:homepage_txs', {
    queryOptions: {
      enabled: feature.isEnabled,
    },
  });

  const { colorMode } = useColorMode();

  if (!feature.isEnabled) {
    return null;
  }

  if (latestTxsQuery.isPending) {
    return <ContentLoader/>;
  }

  const latestTxHash = latestTxsQuery.data?.[0]?.hash;

  const initialQuery = latestTxHash ? `{
    transaction(
      hash: "${ latestTxHash }"
    ) {
      hash
      blockNumber
      value
      gasUsed
    }
  }` : `{
    address(
      hash: "${ ZERO_ADDRESS }"
    ) {
      hash
      fetchedCoinBalance
    }
  }`;

  const graphqlUrl = buildUrl('general:graphql');

  const fetcher = createGraphiQLFetcher({
    url: graphqlUrl,
    // graphql ws implementation with absinthe plugin is incompatible with graphiql-ws protocol
    // or the older one subscriptions-transport-ws
    // so we (@isstuev & @vbaranov) decided to configure playground without subscriptions
    // in case of any complaint consider reconfigure the graphql ws server with absinthe_graphql_ws package
    // subscriptionUrl: `wss://${config.app.host}/socket/`,
  });

  return (
    <Box
      h="100dvh"
      overflowX="auto"
      role="region"
      aria-label="GraphQL query editor"
      tabIndex={ 0 }
      _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-border-widths-2)' }}
      css={ graphQLStyle }
    >
      <Box h="100%" minW="4xl" css={ graphQLStyle }>
        <GraphiQL fetcher={ fetcher } defaultQuery={ initialQuery } forcedTheme={ colorMode }/>
      </Box>
    </Box>
  );
};

export default GraphQL;
