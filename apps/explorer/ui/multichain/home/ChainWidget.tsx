import { Box, HStack, VStack } from '@chakra-ui/react';
import React from 'react';

import type * as multichain from '@blockscout/multichain-aggregator-types';
import type { ClusterChainConfig } from 'types/multichain';

import useAddChainClick from 'lib/web3/useAddChainClick';
import useProvider from 'lib/web3/useProvider';
import { WALLETS_INFO } from 'lib/web3/wallets';
import { Heading } from 'toolkit/chakra/heading';
import { IconButton } from 'toolkit/chakra/icon-button';
import { LinkBox, LinkOverlay } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tooltip } from 'ui/marketplace/NativeTooltip';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import ChainIcon from 'ui/shared/externalChains/ChainIcon';
import IconSvg from 'ui/shared/IconSvg';
import RollupStageBadge from 'ui/snippets/navigation/RollupStageBadge';

interface Props {
  data: ClusterChainConfig;
  isLoading: boolean;
  metrics: multichain.ChainMetrics | undefined;
}

const ChainWidget = ({ data, isLoading, metrics }: Props) => {
  const { data: { wallet } = {} } = useProvider();
  const walletIcon = wallet ? WALLETS_INFO[wallet].icon : undefined;
  const handleAddToWalletClick = useAddChainClick({ source: 'Chain widget' });

  const chainStats = (
    <VStack gap={ 2 } alignItems="flex-start" fontWeight="medium">
      <HStack gap={ 2 }>
        <Skeleton loading={ isLoading } color="text.secondary">
          <span>Chain ID</span>
        </Skeleton>
        <Skeleton loading={ isLoading }>{ data.id }</Skeleton>
        <CopyToClipboard text={ String(data.id) } ml={ 0 } isLoading={ isLoading }/>
      </HStack>
      { metrics?.active_accounts?.current_full_week && (
        <HStack gap={ 2 }>
          <Skeleton loading={ isLoading } color="text.secondary">
            <span>Active accounts</span>
          </Skeleton>
          <Skeleton loading={ isLoading }>{ Number(metrics.active_accounts.current_full_week).toLocaleString() }</Skeleton>
        </HStack>
      ) }
      { metrics?.tps && (
        <HStack gap={ 2 }>
          <Skeleton loading={ isLoading } color="text.secondary">
            <span>TPS</span>
          </Skeleton>
          <Skeleton loading={ isLoading }>{ metrics.tps }</Skeleton>
        </HStack>
      ) }
    </VStack>
  );

  return (
    <LinkBox
      as="article"
      minW={ 0 }
      containerType="inline-size"
      className="group"
      bgColor="surface"
      borderRadius="xl"
      border="var(--chakra-border-widths-1) solid"
      borderColor="hairline"
      _hover={ !isLoading ? {
        bgColor: 'surface',
        borderColor: 'hover',
      } : undefined }
      p={ 6 }
      flex="1 1 var(--chakra-spacing-65)"
      textStyle="sm"
      overflow="hidden"
    >
      <HStack justifyContent="space-between">
        <ChainIcon data={ data } boxSize="calc(var(--chakra-spacing-1) * 7.5)" isLoading={ isLoading } noTooltip/>
        { walletIcon && (
          <Tooltip content="Add to wallet">
            <IconButton
              aria-label={ `Add ${ data.name } to wallet` }
              onClick={ handleAddToWalletClick }
              size="md"
              variant="icon_background"
              zIndex={ 1 }
              bgColor="surface"
              loadingSkeleton={ isLoading }
            >
              <IconSvg name={ walletIcon } boxSize={ 5 }/>
            </IconButton>
          </Tooltip>
        ) }
      </HStack>
      <Heading my={ 3 } textStyle="heading.md" as="h3">
        <LinkOverlay href={ data.explorer_url } external loading={ isLoading } _groupHover={{ color: 'hover' }}>
          <Box overflow="hidden" textOverflow="ellipsis" whiteSpace="nowrap">
            { data.name }
          </Box>
        </LinkOverlay>
      </Heading>
      <RollupStageBadge chainConfig={ data.app_config } isLoading={ isLoading } mb={ 2.5 }/>
      { chainStats }
    </LinkBox>
  );
};

export default React.memo(ChainWidget);
