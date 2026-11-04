import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { ColorThemeId } from 'types/settings';

import { COLOR_THEMES } from 'lib/settings/colorTheme';

import SettingsSample from './SettingsSample';
import { useColorTheme } from './useColorTheme';

interface Props {
  onSelect?: () => void;
}

const SettingsColorTheme = ({ onSelect }: Props) => {
  const { activeThemeId, setTheme } = useColorTheme();

  const handleSelect = React.useCallback((event: React.MouseEvent<HTMLDivElement>) => {
    event.stopPropagation();

    const themeId = event.currentTarget.getAttribute('data-value') as ColorThemeId | null;

    if (!themeId) {
      return;
    }

    setTheme(themeId);
    onSelect?.();
  }, [ setTheme, onSelect ]);

  const activeTheme = COLOR_THEMES.find((theme) => theme.id === activeThemeId);

  return (
    <div>
      <Box fontWeight={ 600 }>Color theme</Box>
      <Box color="text.secondary" mt={ 1 } mb={ 2 }>{ activeTheme?.label }</Box>
      <Flex>
        { COLOR_THEMES.map((theme) => {
          return (
            <SettingsSample
              key={ theme.label }
              label={ theme.label }
              value={ theme.id }
              bg={ theme.sampleBg }
              isActive={ theme.id === activeThemeId }
              onClick={ handleSelect }
            />
          );
        }) }
      </Flex>
    </div>
  );
};

export default React.memo(SettingsColorTheme);
