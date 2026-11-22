import React from 'react';

import { Button } from 'toolkit/chakra/button';
import { PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import IconSvg from 'ui/shared/IconSvg';
import NetworkMenuContent from 'ui/snippets/networkMenu/NetworkMenuContent';
import useNetworkMenu from 'ui/snippets/networkMenu/useNetworkMenu';

const NetworkMenu = () => {
  const menu = useNetworkMenu();

  const handlePopoverOpenChange = React.useCallback(({ open }: { open: boolean }) => {
    menu.onOpenChange({ open });
  }, [ menu ]);

  return (
    <PopoverRoot
      positioning={{ placement: 'bottom-start', offset: { mainAxis: 6 } }}
      lazyMount
      open={ menu.open }
      onOpenChange={ handlePopoverOpenChange }
    >
      <PopoverTrigger>
        <Button
          variant="link"
          aria-label="Network menu"
          onClick={ menu.onToggle }
          size="2xs"
          fontWeight="medium"
        >
          <IconSvg aria-hidden={ true } name="networks" boxSize={ 'var(--chakra-spacing-3\\.5)' }/>
          <span>Chains</span>
          <IconSvg aria-hidden={ true } name="arrows/east-mini" boxSize={ 4 } transform="rotate(-90deg)"/>
        </Button>
      </PopoverTrigger>
      <NetworkMenuContent items={ menu.data } tabs={ menu.availableTabs }/>
    </PopoverRoot>
  );
};

export default React.memo(NetworkMenu);
