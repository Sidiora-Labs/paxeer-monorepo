'use client';

import { ThemeProvider, useTheme } from 'next-themes';
import type { ThemeProviderProps } from 'next-themes';
import * as React from 'react';

import config from 'configs/app';
import * as cookies from 'lib/cookies';

export interface ColorModeProviderProps extends ThemeProviderProps {}

export type ColorMode = 'light' | 'dark';

export function ColorModeProvider(props: ColorModeProviderProps) {
  const [ defaultTheme ] = React.useState(() =>
    cookies.get(cookies.NAMES.COLOR_MODE) || config.UI.colorTheme.default?.colorMode || 'system',
  );

  return (
    <ThemeProvider
      attribute="class"
      scriptProps={{ 'data-cfasync': 'false' }}
      defaultTheme={ defaultTheme }
      storageKey={ config.UI.colorTheme.storageKey }
      disableTransitionOnChange
      { ...props }
    />
  );
}

export function useColorMode() {
  const { resolvedTheme, setTheme } = useTheme();
  const toggleColorMode = () => {
    setTheme(resolvedTheme === 'dark' ? 'light' : 'dark');
  };
  return {
    colorMode: resolvedTheme as ColorMode,
    setColorMode: setTheme,
    toggleColorMode,
  };
}

export function useColorModeValue<T>(light: T, dark: T) {
  const { colorMode } = useColorMode();
  return colorMode === 'dark' ? dark : light;
}
