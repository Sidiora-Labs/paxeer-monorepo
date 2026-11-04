import type { AppKitNetwork } from '@reown/appkit/networks';
import { createAppKit, useAppKitTheme } from '@reown/appkit/react';
import React from 'react';

import config from 'configs/app';
import { useRollbar } from 'lib/rollbar';
import { chains } from 'lib/web3/chains';
import wagmiConfig from 'lib/web3/wagmiConfig';
import { useColorMode } from 'toolkit/chakra/color-mode';
import colors from 'toolkit/theme/foundations/colors';
import { BODY_TYPEFACE } from 'toolkit/theme/foundations/typography';
import zIndex from 'toolkit/theme/foundations/zIndex';

import WagmiProvider from './WagmiProvider';

const feature = config.features.blockchainInteraction;
const INITIALIZATION_ERROR_MESSAGE = 'Wallet connections are unavailable. Please retry.';

type InitializationState = {
  status: 'ready' | 'disabled';
} | {
  status: 'failed';
  error: unknown;
};

type AppKitOptions = Parameters<typeof createAppKit>[0];

export class ReownInitialization {
  state: InitializationState;
  private options?: AppKitOptions;

  constructor(options?: AppKitOptions) {
    this.options = options;
    this.state = this.initialize();
  }

  initialize(): InitializationState {
    try {
      let options = this.options;
      if (!options) {
        if (!feature.isEnabled || feature.connectorType === 'dynamic') {
          this.state = { status: 'disabled' };
          return this.state;
        }
        if (!wagmiConfig.adapter) {
          throw new Error('Wallet adapter is unavailable');
        }

        options = {
          adapters: [ wagmiConfig.adapter ],
          networks: chains as [AppKitNetwork, ...Array<AppKitNetwork>],
          metadata: {
            name: `${ config.chain.name } explorer`,
            description: `${ config.chain.name } explorer`,
            url: config.app.baseUrl,
            icons: [ config.UI.navigation.icon.default ].filter(Boolean),
          },
          projectId: feature.reown.projectId,
          features: {
            analytics: false,
            email: false,
            socials: [],
            onramp: false,
            swaps: false,
          },
          themeVariables: {
            '--w3m-font-family': `${ BODY_TYPEFACE }, sans-serif`,
            '--w3m-accent': colors.blue[600].value,
            '--w3m-border-radius-master': '2px',
            '--w3m-z-index': zIndex?.modal2?.value,
          },
          featuredWalletIds: feature.reown.featuredWalletIds,
          allowUnsupportedChain: true,
        };
      }

      createAppKit(options);
      this.state = { status: 'ready' };
    } catch (error) {
      this.state = { status: 'failed', error };
    }
    return this.state;
  }
}

const reownInitialization = new ReownInitialization();

interface Props {
  children: React.ReactNode;
  initialization?: ReownInitialization;
}

const InitializedReownProvider = ({ children }: { children: React.ReactNode }) => {
  const { colorMode } = useColorMode();
  const { setThemeMode } = useAppKitTheme();

  React.useEffect(() => {
    setThemeMode(colorMode ?? 'light');
  }, [ colorMode, setThemeMode ]);

  return <WagmiProvider>{ children }</WagmiProvider>;
};

const ReownProvider = ({ children, initialization = reownInitialization }: Props) => {
  const [ state, setState ] = React.useState(initialization.state);
  const rollbar = useRollbar();

  React.useEffect(() => {
    if (state.status === 'failed') {
      rollbar?.error(INITIALIZATION_ERROR_MESSAGE);
    }
  }, [ state, rollbar ]);

  const handleRetry = React.useCallback(() => {
    setState(initialization.initialize());
  }, [ initialization ]);

  if (state.status === 'failed') {
    return (
      <section role="alert">
        <p>{ INITIALIZATION_ERROR_MESSAGE }</p>
        <button type="button" onClick={ handleRetry }>Retry wallet connection</button>
      </section>
    );
  }

  if (state.status === 'disabled') {
    return <WagmiProvider>{ children }</WagmiProvider>;
  }

  return <InitializedReownProvider>{ children }</InitializedReownProvider>;
};

export default React.memo(ReownProvider);
