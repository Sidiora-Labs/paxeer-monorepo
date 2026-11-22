import React from 'react';

import { useSettingsContext } from 'lib/contexts/settings';
import { Switch } from 'toolkit/chakra/switch';

const SettingsLocalTime = () => {
  const settingsContext = useSettingsContext();

  if (!settingsContext) {
    return null;
  }

  const { isLocalTime, toggleIsLocalTime } = settingsContext;

  return (
    <Switch minH="calc(var(--chakra-spacing-8) + var(--chakra-spacing-0\\.5))"
      id="local-time"
      defaultChecked={ isLocalTime }
      onChange={ toggleIsLocalTime }
      direction="rtl"
      justifyContent="space-between"
      w="100%"
    >
      Local time format
    </Switch>
  );
};

export default React.memo(SettingsLocalTime);
