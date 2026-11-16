import { chakra, Box, Flex, Spinner } from '@chakra-ui/react';
import React from 'react';

import delay from 'lib/delay';
import useWeb3AccountWithDomain from 'lib/web3/useAccountWithDomain';
import useWeb3Wallet from 'lib/web3/useWallet';
import { Button } from 'toolkit/chakra/button';
import { IconButton } from 'toolkit/chakra/icon-button';
import { Hint } from 'toolkit/components/Hint/Hint';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  onClose?: () => void;
  className?: string;
}

const UserProfileContentWallet = ({ onClose, className }: Props) => {
  const web3Wallet = useWeb3Wallet({ source: 'Profile dropdown' });

  const web3AccountWithDomain = useWeb3AccountWithDomain(true);

  const handleConnectWalletClick = React.useCallback(async() => {
    web3Wallet.openModal();
    await delay(300);
    onClose?.();
  }, [ web3Wallet, onClose ]);

  const handleOpenWalletClick = React.useCallback(async() => {
    web3Wallet.openModal();
    await delay(300);
    onClose?.();
  }, [ web3Wallet, onClose ]);

  const handleAddressClick = React.useCallback(() => {
    onClose?.();
  }, [ onClose ]);

  const content = (() => {
    if (web3Wallet.isConnected && web3AccountWithDomain.address) {
      return (
        <Flex
          alignItems="center"
          columnGap={ 2 }
          bgColor="surface"
          px={ 2 }
          py={ 'var(--chakra-spacing-2\\.5)' }
          borderRadius="base"
          justifyContent="space-between"
        >
          <AddressEntity
            address={{ hash: web3AccountWithDomain.address, ens_domain_name: web3AccountWithDomain.domain }}
            isLoading={ web3AccountWithDomain.isLoading }
            truncation="dynamic"
            fontSize="sm"
            fontWeight="medium"
            noAltHash
            onClick={ handleAddressClick }
          />
          { web3Wallet.isReconnecting ? <Spinner size="sm" m={ 'var(--chakra-spacing-0\\.5)' } flexShrink={ 0 }/> : (
            <IconButton minH="touchTarget" minW="touchTarget"
              aria-label="Open wallet"
              variant="icon_secondary"
              size="2xs"
              onClick={ handleOpenWalletClick }
              loading={ web3Wallet.isOpen }
            >
              <IconSvg aria-hidden={ true } name="gear"/>
            </IconButton>
          ) }
        </Flex>
      );
    }

    return (
      <Button minH="touchTarget" minW="touchTarget"
        size="sm"
        onClick={ handleConnectWalletClick }
        loading={ web3Wallet.isOpen }
        loadingText="Connect Wallet"
        w="100%"
      >
        Connect
      </Button>
    );
  })();

  return (
    <Box className={ className }>
      <Flex px={ 1 } mb="1" textStyle="xs" alignItems="center" fontWeight="medium">
        <span>Connected wallet</span>
        <Hint
          label={
            web3Wallet.isConnected ?
              'This wallet is currently connected to Blockscout and used for interacting with apps and smart contracts' :
              'This wallet is used for interacting with apps and smart contracts'
          }
          boxSize={ 4 }
          ml={ 1 }
        />
      </Flex>
      { content }
    </Box>
  );
};

export default React.memo(chakra(UserProfileContentWallet));
