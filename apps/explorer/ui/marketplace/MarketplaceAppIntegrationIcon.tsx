import React from 'react';

import { IconButton } from 'toolkit/chakra/icon-button';
import durations from 'toolkit/theme/foundations/durations';
import { Tooltip } from 'ui/marketplace/NativeTooltip';
import type { IconName } from 'ui/shared/IconSvg';
import IconSvg from 'ui/shared/IconSvg';

type Props = {
  internalWallet: boolean | undefined;
  external: boolean | undefined;
};

const MarketplaceAppIntegrationIcon = ({ external, internalWallet }: Props) => {
  const [ icon, iconColor, text, boxSize ] = React.useMemo(() => {
    let icon: IconName = 'integration/partial';
    let color = 'icon.secondary';
    let text = 'This app opens in Blockscout without Blockscout wallet functionality. Use your external web3 wallet to connect directly to this application';
    let boxSize = 5;

    if (external) {
      icon = 'link_external';
      color = 'icon.secondary';
      text = 'This app opens in a separate tab';
      boxSize = 4;
    } else if (internalWallet) {
      icon = 'integration/full';
      color = 'positive';
      text = 'This app opens in Blockscout and your Blockscout wallet connects automatically';
    }

    return [ icon, color, text, boxSize ];
  }, [ external, internalWallet ]);

  return (
    <Tooltip
      content={ text }
      openDelay={ Number.parseInt(durations.slow.value, 10) }
      contentProps={{ maxW: { base: 'calc(100vw - var(--chakra-spacing-2))', lg: 'var(--chakra-spacing-100)' } }}
    >
      <IconButton aria-label={ text } variant="icon_secondary" size="md">
        <IconSvg
          name={ icon }
          boxSize={ boxSize }
          color={ iconColor }
          position="relative"
          cursor="pointer"
          verticalAlign="middle"
        />
      </IconButton>
    </Tooltip>
  );
};

export default MarketplaceAppIntegrationIcon;
