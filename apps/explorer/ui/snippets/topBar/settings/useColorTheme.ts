import React from 'react';

import type { ColorThemeId } from 'types/settings';

import * as cookies from 'lib/cookies';
import { COLOR_THEMES, getDefaultColorTheme, getThemeHexWithOverrides } from 'lib/settings/colorTheme';
import type { ColorMode } from 'toolkit/chakra/color-mode';
import { useColorMode } from 'toolkit/chakra/color-mode';

// Follow the same reactive mode as the top-bar toggle, and read its persisted theme
// whenever that mode changes, including while the settings popover is closed.
export function useColorTheme() {
  const { colorMode, setColorMode } = useColorMode();

  const [ activeThemeId, setActiveThemeId ] = React.useState<ColorThemeId>();

  const setTheme = React.useCallback((themeId: ColorThemeId) => {
    const nextTheme = COLOR_THEMES.find((theme) => theme.id === themeId);
    const varValue = getThemeHexWithOverrides(themeId);

    if (!nextTheme || !varValue) {
      return;
    }

    setColorMode(nextTheme.colorMode);

    const varName = nextTheme.colorMode === 'light' ? '--chakra-colors-white' : '--chakra-colors-black';
    const varNameBg = nextTheme.colorMode === 'light' ? '--chakra-colors-theme-bg-primary-_light' : '--chakra-colors-theme-bg-primary-_dark';
    window.document.documentElement.style.setProperty(varName, varValue);
    window.document.documentElement.style.setProperty(varNameBg, varValue);

    cookies.set(cookies.NAMES.COLOR_MODE, nextTheme.colorMode);
    cookies.set(cookies.NAMES.COLOR_THEME, themeId);
    window.localStorage.setItem(cookies.NAMES.COLOR_MODE, nextTheme.colorMode);
    setActiveThemeId(themeId);
  }, [ setColorMode ]);

  React.useEffect(() => {
    const cookieColorMode = cookies.get(cookies.NAMES.COLOR_MODE) as ColorMode | undefined;
    const cookieColorTheme = cookies.get(cookies.NAMES.COLOR_THEME) as ColorThemeId | undefined;

    const nextColorMode = colorMode || cookieColorMode ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');

    const savedTheme = COLOR_THEMES.find((theme) => theme.id === cookieColorTheme && theme.colorMode === nextColorMode);
    const nextColorTheme = savedTheme?.id || getDefaultColorTheme(nextColorMode);

    setTheme(nextColorTheme);
  }, [ colorMode, setTheme ]);

  return { activeThemeId, setTheme };
}
