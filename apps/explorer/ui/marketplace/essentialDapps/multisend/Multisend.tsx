import { Box } from '@chakra-ui/react';
import { MultisenderWidget } from '@multisender.app/multisender-react-widget';
import React from 'react';

import { getFeaturePayload } from 'configs/app/features/types';

import config from 'configs/app';
import essentialDappsChainsConfig from 'configs/essential-dapps-chains';
import useIsMobile from 'lib/hooks/useIsMobile';
import AdBanner from 'ui/shared/ad/AdBanner';

const feature = getFeaturePayload(config.features.marketplace);
const dappConfig = feature?.essentialDapps?.multisend;

const Container = ({ children }: { children: React.ReactNode }) => (
  <Box
    w="full"
    maxW="calc(var(--chakra-spacing-1) * 167.5)"
    mx="auto"
    css={{
      '& > .multisenderTheme': {
        '--mw-color-brand': 'colors.accent',
        '--mw-color-brand-stroke': 'colors.hairline',
        '--mw-color-brand-text-secondary': 'colors.text.secondary',
        '--mw-color-brand-fill': 'colors.dialog.bg', // modal background
        fontFamily: 'body',
        '& [class*="_stepBody_"] > span': {
          fontSize: 'xs',
          fontWeight: 'normal',
        },
        '& [class*="_step_"]:not([data-progress="true"]), & [class*="_step_"][data-completed="true"]': {
          '& [class*="_stepBody_"] > span': {
            color: 'text.secondary',
          },
          '& [class*="_stepIcon_"]': {
            backgroundColor: 'raised',
          },
          '& [class*="_stepCompletedIcon_"]': {
            color: 'text.secondary',
          },
        },
        '& [class*="_stepIcon_"]': {
          fontSize: 'xs',
          width: 'var(--chakra-spacing-6)',
          height: 'var(--chakra-spacing-6)',
          minWidth: 'var(--chakra-spacing-6)',
          minHeight: 'var(--chakra-spacing-6)',
        },
        '& [class*="_stepSeparator_"]': {
          height: 'calc(var(--chakra-spacing-1) * 0.25)',
        },
        '& [class*="_stepperRoot_"] > :nth-child(1)': {
          padding: '0 var(--chakra-spacing-3)',
        },
        '& [class*="_fieldsetRoot_"]:not([class*="_itemRoot_"])': {
          borderRadius: 'md',
          padding: 'var(--chakra-spacing-6)',
          '& button': {
            fontSize: 'xs',
            '& label': {
              fontWeight: 'medium !important',
            },
          },
          '& > *:first-child': {
            gap: 'var(--chakra-spacing-3)',
            '& > :nth-child(4) > button': {
              marginBottom: 'var(--chakra-spacing-3)',
              border: 'var(--chakra-border-widths-2) solid',
              borderColor: 'button.outline.fg',
              borderRadius: 'base',
              color: 'button.outline.fg',
              height: 'var(--chakra-spacing-8)',
              padding: '0 var(--chakra-spacing-3)',
              fontSize: 'sm',
              transitionProperty: 'all',
              transitionDuration: 'normal',
              '&:hover': {
                background: 'none',
                color: 'hover',
                borderColor: 'hover',
              },
              '&:active': {
                transform: 'none',
              },
            },
          },
          '& > *:last-child': {
            '& p[data-size="sm"], & a[data-size="sm"]': {
              fontSize: 'xs',
            },
            '& a[data-size="sm"]': {
              color: 'link.primary',
              fontWeight: 'medium !important',
              '&:hover': {
                color: 'link.primary.hover',
              },
            },
          },
          '& p[data-size="xl"], & h5': {
            fontFamily: 'heading',
            fontSize: 'headingMd',
            fontWeight: 'medium !important',
          },
          '& [class*="_tableTr_"]': {
            fontWeight: 'medium',
            '& td': {
              padding: 'var(--chakra-spacing-3) 0',
            },
            '& a': {
              color: 'link.primary',
              fontSize: 'sm',
              '&:hover': {
                color: 'link.primary.hover',
                textDecoration: 'none',
              },
            },
          },
          '& [class*="SummaryList-module__root__"]': {
            '& + div': {
              gap: 'var(--chakra-spacing-1)',
              '& > div:last-child': {
                '& button': {
                  background: 'none',
                  border: 'none',
                  color: 'button.outline.fg',
                  transitionProperty: 'all',
                  transitionDuration: 'normal',
                  '&:hover': {
                    color: 'hover',
                  },
                  '&:active': {
                    transform: 'none',
                  },
                },
                '& p': {
                  fontSize: 'sm',
                  color: 'text.secondary',
                },
              },
            },
            '& + button': {
              marginTop: 'var(--chakra-spacing-3)',
            },
            '& span': {
              '--loader-color': 'var(--chakra-colors-text-primary) !important',
            },
          },
        },
        '& [class*="_itemRoot_"]': {
          border: 'none',
          borderRadius: 'base',
          backgroundColor: 'surface',
          '& > div': {
            gap: 'var(--chakra-spacing-2)',
          },
        },
        '& [class*="_inputWrapper_"]': {
          '& [class*="_input_"]': {
            borderRadius: 'base',
            border: 'var(--chakra-border-widths-2) solid',
            borderColor: 'input.border.filled',
            height: 'var(--chakra-spacing-8)',
            minHeight: 'var(--chakra-spacing-8)',
            fontSize: 'sm',
            fontWeight: 'medium',
            color: 'input.fg',
            '&::placeholder': {
              color: 'input.placeholder',
            },
          },
          '&[data-size="xl"]': {
            '& [class*="_input_"]': {
              height: 'var(--chakra-spacing-15)',
              minHeight: 'var(--chakra-spacing-15)',
              fontSize: 'md',
            },
            '& button': {
              height: 'var(--chakra-spacing-10)',
              fontSize: 'md',
              backgroundColor: 'button.solid.bg',
              borderRadius: 'base',
              transitionProperty: 'all',
              transitionDuration: 'normal',
              color: 'onAccent',
              '&:hover': {
                backgroundColor: 'hover',
              },
              '&:active': {
                transform: 'none',
              },
              '&:disabled': {
                opacity: '0.2',
              },
            },
          },
        },
        '& [class*="RecipientsEditor-module__head_"]': {
          fontSize: 'xs',
          marginTop: 'var(--chakra-spacing-3)',
          '& p': {
            fontWeight: 'medium',
          },
          '& button': {
            background: 'none',
            color: 'link.primary',
            fontWeight: 'medium',
            padding: 0,
            height: 'calc(var(--chakra-spacing-1) * 4.5)',
            transitionProperty: 'all',
            transitionDuration: 'normal',
            borderRadius: 0,
            '&:hover': {
              color: 'link.primary.hover',
            },
            '&:active': {
              transform: 'none',
            },
          },
        },
        '& [class*="CodeMirror"]': {
          fontSize: 'body',
          '& .cm-scroller': {
            borderRadius: 'base',
            borderWidth: '2',
            borderColor: 'input.border.filled',
          },
          '& .cm-gutters': {
            borderTopLeftRadius: '0',
            borderBottomLeftRadius: '0',
            borderRightWidth: 'calc(var(--chakra-spacing-1) * 0.5)',
            borderColor: 'input.border.filled',
            backgroundColor: 'surface',
            backdropFilter: 'none',
            '&:before': {
              background: 'none',
            },
          },
          '& .cm-placeholder': {
            color: 'input.placeholder',
          },
        },
        '& [class*="_modalRoot_"]': {
          '& [class*="_modalOverlay_"]': {
            backgroundColor: 'blackAlpha.800',
          },
          '& [class*="_modalContent_"]': {
            borderRadius: 'xl',
          },
          '& [class*="_modalHeader_"]': {
            minHeight: 'var(--chakra-spacing-16)',
            height: 'var(--chakra-spacing-16) !important',
            padding: 'var(--chakra-spacing-6) !important',
            paddingBottom: '0 !important',
            marginBottom: 'var(--chakra-spacing-2)',
            '& [class*="_modalTitle_"]': {
              fontFamily: 'heading',
              fontWeight: 'medium !important',
              fontSize: 'headingLg !important',
            },
          },
          '& [class*="_modalHeader_"] + div': {
            padding: 'var(--chakra-spacing-6) !important',
            paddingTop: '0 !important',
            '& > div': {
              padding: '0 !important',
            },
            '& h6, & p': {
              fontSize: 'md',
            },
          },
          '& [class*="_closeButton_"]': {
            background: 'none',
            color: 'close-button-fg',
            '&:hover': {
              color: 'hover',
            },
          },
        },
        '& div[role="alert"]': {
          borderRadius: 'base',
          padding: 'var(--chakra-spacing-2) var(--chakra-spacing-3)',
          margin: '0 !important',
          '& p, & div': {
            fontSize: 'md',
            color: 'text.primary !important',
          },
          '& [class*="_alertIcon_"]': {
            marginTop: 'calc(var(--chakra-spacing-1) * 0.5)',
            marginRight: 'var(--chakra-spacing-2)',
            '& svg': {
              fill: 'text.primary',
              width: 'var(--chakra-spacing-5)',
              height: 'var(--chakra-spacing-5)',
            },
          },
          '&:not([data-variant="filled"]) [class*="_alertIcon_"]': {
            display: 'none',
          },
          '&[data-variant="filled"]': {
            backgroundColor: 'alert.bg.info !important',
          },
        },
        '& div[role="tooltip"]': {
          backgroundColor: 'tooltip.bg',
          color: 'tooltip.fg',
          borderRadius: 'sm',
          padding: 'var(--chakra-spacing-1) var(--chakra-spacing-2) !important',
          '& p': {
            fontSize: 'sm',
            fontWeight: 'medium',
          },
        },
        '& [class*="_button_"][data-size="lg"], [class*="_button_"][data-size="md"]': {
          borderRadius: 'base',
          color: 'onAccent',
          height: 'var(--chakra-spacing-10)',
          backgroundColor: 'button.solid.bg',
          transitionProperty: 'background-color',
          transitionDuration: 'normal',
          '&:disabled': {
            opacity: '0.2',
          },
          '&:hover': {
            backgroundColor: 'hover',
          },
          '&:active': {
            transform: 'none',
          },
          '& > span[aria-hidden="true"]': {
            width: 'var(--chakra-spacing-5)',
            height: 'var(--chakra-spacing-5)',
            '& > span::after': {
              width: 'var(--chakra-spacing-5)',
              height: 'var(--chakra-spacing-5)',
            },
          },
        },
        '& [class*="Summary-module__upgradeBtn__"]': {
          borderRadius: 'base',
          border: 'none',
          height: 'var(--chakra-spacing-8)',
          padding: '0 var(--chakra-spacing-3)',
          fontSize: 'sm !important',
          fontWeight: 'semibold',
          transitionProperty: 'all',
          transitionDuration: 'normal',
          '&[class*="_crown_"]': {
            color: 'onAccent',
            backgroundColor: 'button.solid.bg',
            '&:hover': {
              backgroundColor: 'hover',
            },
          },
          '&[class*="_gift_"]': {
            color: 'button.outline.fg',
            border: 'var(--chakra-border-widths-2) solid',
            borderColor: 'button.outline.fg',
            marginRight: 'calc(var(--chakra-spacing-1) * -1)',
            background: 'none',
            '&:hover': {
              color: 'hover',
              borderColor: 'hover',
            },
          },
          '& span[data-position="left"]': {
            display: 'none',
          },
        },
        '& [class*="Summary-module__editGasCost__"]': {
          border: 'none',
          background: 'none',
          '& svg': {
            width: 'var(--chakra-spacing-4)',
            height: 'var(--chakra-spacing-4)',
            color: 'link.primary',
            transitionProperty: 'all',
            transitionDuration: 'normal',
          },
          '&:hover svg': {
            color: 'link.primary.hover',
          },
          '&:active': {
            transform: 'none',
          },
        },
        '& div[role="radiogroup"]': {
          '& > div': {
            gap: 'var(--chakra-spacing-2)',
          },
          '& [class*="LargeRadio-module__root_"]': {
            '& svg': {
              transform: 'none',
            },
            '& label': {
              '& p:not([data-size="sm"])': {
                fontWeight: 'semibold !important',
              },
              '& p[data-size="sm"]': {
                color: 'text.secondary',
              },
            },
            '& > div[class*="LargeRadio-module__body_"]': {
              border: 'none',
              borderRadius: 'base',
              backgroundColor: 'surface',
            },
            '&:not([data-checked="true"]) input': {
              backgroundColor: 'transparent',
              border: 'var(--chakra-border-widths-2) solid',
              borderColor: 'input.border.filled',
            },
            '& > div > div:not([class*="_labelWrapper_"])': {
              marginTop: 'calc(var(--chakra-spacing-1) * 0.5)',
            },
            '& .tabler-icon-info-circle': {
              stroke: 'text.primary',
              width: 'var(--chakra-spacing-5)',
              height: 'var(--chakra-spacing-5)',
            },
          },
        },
        '& [class*="TokenSelect-module__grid_"]': {
          display: 'flex',
          justifyContent: 'flex-start',
          gap: 'var(--chakra-spacing-3)',
          flexWrap: 'wrap',
        },
        '& [class*="TokenSelect-module__chainsList_"]': {
          gap: 0,
          marginTop: 'var(--chakra-spacing-6)',
          '& > div': {
            display: 'flex',
            padding: 'var(--chakra-spacing-2)',
            margin: '0 calc(var(--chakra-spacing-2) * -1)',
            borderRadius: 'base',
            cursor: 'pointer',
            '&:hover': {
              backgroundColor: 'selected.control.bg',
            },
            '& > [class*="_chainRow_"]': {
              flex: 1,
              '&:hover': {
                color: 'inherit',
              },
            },
            '& > svg': {
              width: 'calc(var(--chakra-spacing-1) * 4.5)',
            },
          },
        },
        '& [class*="TokenSelect-module__stickySection_"]': {
          top: 'var(--chakra-spacing-4)',
          '& > div:last-child': {
            gap: 'var(--chakra-spacing-2) !important',
          },
        },
        '& [class*="TokenSelect-module__searchInput_"]': {
          marginTop: 'var(--chakra-spacing-2)',
        },
        '& [class*="TokenSelect-module__tokensList_"]': {
          gap: 0,
          '& svg > path:first-child': {
            stroke: 'text.primary',
          },
        },
        '& [class*="TokenSelect-module__tokenRow_"]': {
          padding: 'var(--chakra-spacing-2)',
          margin: '0 calc(var(--chakra-spacing-2) * -1)',
          borderRadius: 'base',
          '&:hover': {
            backgroundColor: 'selected.control.bg',
          },
          '& [class*="_tokenIcon_"]': {
            width: 'var(--chakra-spacing-10)',
            height: 'var(--chakra-spacing-10)',
          },
        },
        '& > .multisenderMantineProvider': {
          '--mantine-color-text': 'text.primary',
          '--mantine-color-red-light': 'colors.alert.bg.error',
          '--mantine-color-blue-light': 'colors.alert.bg.info',
          '--mantine-color-blue-light-color': 'colors.text.secondary',
          '--mantine-color-blue-text': 'colors.hover',
          '--mantine-color-blue-outline': 'colors.hover',
          '--mantine-color-green-text': 'colors.green.500',
          '& > div:nth-child(1)': {
            padding: '0 !important',
            gap: 0,
            '& > :nth-child(1)': {
              margin: '0 !important',
            },
          },
        },
      },
    }}
  >{ children }</Box>
);

const widgetConfig = Object.fromEntries(dappConfig?.chains.map((chainId) => {
  const chainConfig = essentialDappsChainsConfig()?.chains.find((chain) => chain.id === chainId);
  const explorerUrl = chainConfig?.app_config?.app?.baseUrl;
  const apiUrl = chainConfig?.app_config?.apis?.general?.endpoint;
  return [
    chainId,
    {
      id: Number(chainId),
      blockExplorerUrl: {
        tx: `${ explorerUrl }/tx/`,
        address: `${ explorerUrl }/address/`,
      },
      rpcUrls: [ `${ apiUrl }/api/eth-rpc` ],
      blockScoutApiUrl: apiUrl,
    },
  ];
}) || []);

const Multisend = () => {
  const isMobile = useIsMobile();

  return (
    <>
      <Container>
        <MultisenderWidget
          config={ widgetConfig }
          logoType="minified"
          posthogKey={ dappConfig?.posthogKey }
          posthogHost={ dappConfig?.posthogHost }
          classNames={{
            theme: 'multisenderTheme',
            mantineProvider: 'multisenderMantineProvider',
          }}
        />
      </Container>
      { (feature?.essentialDappsAdEnabled && !isMobile) && (
        <AdBanner
          format="desktop"
          w="fit-content"
          borderRadius="md"
          overflow="hidden"
          mx="auto"
          mt={ 10 }
        />
      ) }
    </>
  );
};

export default Multisend;
