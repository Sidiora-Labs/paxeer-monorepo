import { HStack } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { useAppContext } from 'lib/contexts/app';
import * as cookies from 'lib/cookies';
import useIsMobile from 'lib/hooks/useIsMobile';
import useProvider from 'lib/web3/useProvider';
import RewardsButton from 'ui/rewards/RewardsButton';
import NetworkAddToWallet from 'ui/shared/NetworkAddToWallet';
import RollupStageBadge from 'ui/snippets/navigation/RollupStageBadge';
import TestnetBadge from 'ui/snippets/navigation/TestnetBadge';

import ColorModeToggle from './ColorModeToggle';
import DeFiDropdown from './DeFiDropdown';
import NetworkMenu from './NetworkMenu';
import Settings from './settings/Settings';

const TopBar = () => {
  const hideAddToWalletButtonCookie = cookies.get(cookies.NAMES.HIDE_ADD_TO_WALLET_BUTTON, useAppContext().cookies);
  const [ isAddChainButtonVisible, setIsAddChainButtonVisible ] = React.useState(hideAddToWalletButtonCookie !== 'topbar');

  const web3 = useProvider();
  const isMobile = useIsMobile();

  const hasAddChainButton = Boolean(
    isAddChainButtonVisible &&
    web3.data?.provider &&
    web3.data?.wallet &&
    config.chain.rpcUrls.length &&
    config.features.web3Wallet.isEnabled &&
    !config.features.multichain.isEnabled &&
    !isMobile,
  );
  const hasDeFiDropdown = Boolean(config.features.deFiDropdown.isEnabled);
  const hasNetworkMenu = Boolean(config.UI.featuredNetworks.items || config.features.multichain.isEnabled);
  const handleAddSuccess = React.useCallback(() => {
    cookies.set(cookies.NAMES.HIDE_ADD_TO_WALLET_BUTTON, 'topbar', { expires: 3 * 365 });
    setIsAddChainButtonVisible(false);
  }, [ ]);

  return (
    <HStack data-label="header-utilities" gap={ 2 } flexShrink={ 0 }>
      <Settings>
        <HStack flexWrap="wrap" gap={ 3 }>
          <TestnetBadge/>
          <RollupStageBadge/>
          { hasNetworkMenu && <NetworkMenu/> }
          { hasAddChainButton && <NetworkAddToWallet source="Top bar" onAddSuccess={ handleAddSuccess }/> }
          { hasDeFiDropdown && <DeFiDropdown/> }
          { config.features.rewards.isEnabled && <RewardsButton size="sm"/> }
        </HStack>
      </Settings>
      <ColorModeToggle/>
    </HStack>
  );
};

export default React.memo(TopBar);
