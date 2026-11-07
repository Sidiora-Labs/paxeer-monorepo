import React from 'react';

import { DrawerBody, DrawerContent, DrawerRoot, DrawerTrigger, DrawerHeader, DrawerTitle, DrawerCloseTrigger } from 'toolkit/chakra/drawer';
import { IconButton } from 'toolkit/chakra/icon-button';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';
import NavigationMobile from 'ui/snippets/navigation/mobile/NavigationMobile';
import RollupStageBadge from 'ui/snippets/navigation/RollupStageBadge';
import TestnetBadge from 'ui/snippets/navigation/TestnetBadge';
import NetworkLogo from 'ui/snippets/networkLogo/NetworkLogo';

interface Props {
  isMarketplaceAppPage?: boolean;
}

const Burger = ({ isMarketplaceAppPage }: Props) => {
  const { open, onOpen, onClose, onOpenChange } = useDisclosure();

  return (
    <DrawerRoot
      open={ open }
      onOpenChange={ onOpenChange }
      placement="start"
      lazyMount={ false }
    >
      <DrawerTrigger>
        <IconButton onClick={ onOpen } p={ 2 } aria-label="Menu button">
          <IconSvg
            name="burger"
            boxSize={ 6 }
            display="block"
            color="text.primary"
          />
        </IconButton>
      </DrawerTrigger>
      <DrawerContent bg="raised" borderColor="hairline" borderWidth="1px">
        <DrawerHeader>
          <DrawerTitle>Navigation</DrawerTitle>
          <DrawerCloseTrigger/>
        </DrawerHeader>
        <DrawerBody display="flex" flexDirection="column" overflowX="hidden" overflowY="auto">
          <TestnetBadge alignSelf="flex-start" mb={ 2 }/>
          <RollupStageBadge alignSelf="flex-start" mb={ 2 }/>
          <NetworkLogo/>
          <NavigationMobile onNavLinkClick={ onClose } isMarketplaceAppPage={ isMarketplaceAppPage }/>
        </DrawerBody>
      </DrawerContent>
    </DrawerRoot>
  );
};

export default Burger;
