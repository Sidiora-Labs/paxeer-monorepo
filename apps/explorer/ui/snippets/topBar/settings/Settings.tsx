import { Flex, Separator, VStack } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { IconButton } from 'toolkit/chakra/icon-button';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import { Tooltip } from 'toolkit/chakra/tooltip';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';

import TopBarStats from '../TopBarStats';
import SettingsAddressFormat from './SettingsAddressFormat';
import SettingsColorTheme from './SettingsColorTheme';
import SettingsIdentIcon from './SettingsIdentIcon';
import SettingsLocalTime from './SettingsLocalTime';
import SettingsPoorReputationTokens from './SettingsPoorReputationTokens';
import SettingsScamTokens from './SettingsScamTokens';

const Settings = ({ children }: { children?: React.ReactNode }) => {
  const popover = useDisclosure();
  const tooltip = useDisclosure();

  const handlePopoverOpenChange = React.useCallback(({ open }: { open: boolean }) => {
    open && tooltip.onClose();
    popover.onOpenChange({ open });
  }, [ popover, tooltip ]);

  const handleTooltipOpenChange = React.useCallback(({ open }: { open: boolean }) => {
    if (!popover.open) {
      tooltip.onOpenChange({ open });
    }
  }, [ popover, tooltip ]);

  return (
    <PopoverRoot
      positioning={{ placement: 'bottom-start' }}
      open={ popover.open }
      onOpenChange={ handlePopoverOpenChange }
      // should be false to enable auto-switch to default color theme
      lazyMount={ false }
    >
      <Tooltip content="Website settings" disableOnMobile open={ tooltip.open } onOpenChange={ handleTooltipOpenChange }>
        <Flex alignItems="center">
          <PopoverTrigger>
            <IconButton
              variant="link"
              size="2xs"
              borderRadius="sm"
              aria-label="User settings"
            >
              <IconSvg name="gear"/>
            </IconButton>
          </PopoverTrigger>
        </Flex>
      </Tooltip>
      <PopoverContent overflowY="hidden" w="auto" fontSize="sm">
        <PopoverBody>
          { !config.features.multichain.isEnabled && (
            <Flex data-label="chain-stats" flexWrap="wrap" gap={ 2 } pb={ 3 } mb={ 3 } borderBottomWidth="1px" borderColor="hairline">
              <TopBarStats/>
            </Flex>
          ) }
          { children && <Flex mb={ 3 }>{ children }</Flex> }
          <SettingsColorTheme onSelect={ popover.onClose }/>
          <Separator my={ 3 }/>
          <SettingsIdentIcon/>
          <SettingsAddressFormat/>
          <Separator my={ 3 }/>
          <VStack gap={ 1 }>
            <SettingsScamTokens/>
            <SettingsPoorReputationTokens/>
            <SettingsLocalTime/>
          </VStack>
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(Settings);
