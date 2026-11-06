import type { ThemingConfig } from '@chakra-ui/react';

import config from 'configs/app';

const heroBannerButton = config.UI.homepage.heroBanner?.button;

function roleColor(role: string) {
  return { value: { _light: `{colors.theme.${ role }._light}`, _dark: `{colors.theme.${ role }._dark}` } };
}

function aliasColor(token: string) {
  return { value: { _light: `{colors.${ token }}`, _dark: `{colors.${ token }}` } };
}

const semanticTokens: ThemingConfig['semanticTokens'] = {
  colors: {
    canvas: roleColor('canvas'),
    surface: roleColor('surface'),
    raised: roleColor('raised'),
    hairline: roleColor('hairline'),
    onAccent: roleColor('onAccent'),
    positive: roleColor('positive'),
    positiveSoft: roleColor('positiveSoft'),
    negative: roleColor('negative'),
    negativeSoft: roleColor('negativeSoft'),
    warning: roleColor('warning'),
    warningSoft: roleColor('warningSoft'),
    headerBar: roleColor('headerBar'),
    focusRing: roleColor('focusRing'),
    sparkline: {
      up: roleColor('sparklineUp'),
      down: roleColor('sparklineDown'),
    },
    chart: {
      grid: roleColor('chartGrid'),
      area: {
        start: roleColor('chartAreaStart'),
        stop: roleColor('chartAreaStop'),
      },
    },
    graph: {
      line: aliasColor('accent'),
      gradient: { start: aliasColor('chart.area.start'), stop: aliasColor('chart.area.stop') },
    },
    graphStart: roleColor('graphStart'),
    graphStop: roleColor('graphStop'),
    success: aliasColor('positive'),
    successSoft: aliasColor('positiveSoft'),
    destructive: aliasColor('negative'),
    destructiveSoft: aliasColor('negativeSoft'),
    tonal: aliasColor('accent.soft'),
    outline: aliasColor('hairline'),
    dataTableBorder: aliasColor('hairline'),
    selectedOption: aliasColor('accent.soft'),
    transparent: { value: { _light: 'transparent', _dark: 'transparent' } },
    // COMMON STATES
    hover: {
      DEFAULT: { value: { _light: '{colors.theme.hover._light}', _dark: '{colors.theme.hover._dark}' } },
    },
    selected: {
      control: {
        bg: { value: { _light: '{colors.theme.selected.control.bg._light}', _dark: '{colors.theme.selected.control.bg._dark}' } },
        text: { value: { _light: '{colors.theme.selected.control.text._light}', _dark: '{colors.theme.selected.control.text._dark}' } },
      },
      option: {
        bg: { value: { _light: '{colors.theme.selected.option.bg._light}', _dark: '{colors.theme.selected.option.bg._dark}' } },
      },
    },

    // GLOBAL
    global: {
      body: {
        bg: { value: { _light: '{colors.bg.primary}', _dark: '{colors.bg.primary}' } },
        fg: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
      },
      mark: {
        bg: { value: { _light: '{colors.green.100}', _dark: '{colors.green.800}' } },
      },
      scrollbar: {
        thumb: { value: { _light: '{colors.blackAlpha.300}', _dark: '{colors.whiteAlpha.300}' } },
      },
      selection: {
        bg: aliasColor('accent.soft'),
      },
    },

    accent: {
      DEFAULT: { value: { _light: '{colors.theme.accent.primary._light}', _dark: '{colors.theme.accent.primary._dark}' } },
      strong: { value: { _light: '{colors.theme.accent.strong._light}', _dark: '{colors.theme.accent.strong._dark}' } },
      soft: { value: { _light: '{colors.theme.accent.soft._light}', _dark: '{colors.theme.accent.soft._dark}' } },
    },
    feedback: {
      success: {
        fg: { value: { _light: '{colors.theme.feedback.success.fg._light}', _dark: '{colors.theme.feedback.success.fg._dark}' } },
        bg: { value: { _light: '{colors.theme.feedback.success.bg._light}', _dark: '{colors.theme.feedback.success.bg._dark}' } },
      },
      error: {
        fg: { value: { _light: '{colors.theme.feedback.error.fg._light}', _dark: '{colors.theme.feedback.error.fg._dark}' } },
        bg: { value: { _light: '{colors.theme.feedback.error.bg._light}', _dark: '{colors.theme.feedback.error.bg._dark}' } },
      },
      warning: {
        fg: { value: { _light: '{colors.theme.feedback.warning.fg._light}', _dark: '{colors.theme.feedback.warning.fg._dark}' } },
        bg: { value: { _light: '{colors.theme.feedback.warning.bg._light}', _dark: '{colors.theme.feedback.warning.bg._dark}' } },
      },
    },

    // FOUNDATIONS
    heading: {
      DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
    },
    text: {
      primary: { value: { _light: '{colors.theme.text.primary._light}', _dark: '{colors.theme.text.primary._dark}' } },
      secondary: { value: { _light: '{colors.theme.text.secondary._light}', _dark: '{colors.theme.text.secondary._dark}' } },
      muted: { value: { _light: '{colors.theme.text.muted._light}', _dark: '{colors.theme.text.muted._dark}' } },
      error: { value: { _light: '{colors.feedback.error.fg}', _dark: '{colors.feedback.error.fg}' } },
      success: { value: { _light: '{colors.feedback.success.fg}', _dark: '{colors.feedback.success.fg}' } },
    },
    bg: {
      canvas: aliasColor('canvas'),
      raised: aliasColor('raised'),
      headerBar: aliasColor('headerBar'),
      primary: { value: { _light: '{colors.theme.bg.primary._light}', _dark: '{colors.theme.bg.primary._dark}' } },
      surface: { value: { _light: '{colors.theme.bg.surface._light}', _dark: '{colors.theme.bg.surface._dark}' } },
      sunken: { value: { _light: '{colors.theme.bg.sunken._light}', _dark: '{colors.theme.bg.sunken._dark}' } },
      overlay: { value: { _light: '{colors.theme.bg.overlay._light}', _dark: '{colors.theme.bg.overlay._dark}' } },
    },
    border: {
      divider: { value: { _light: '{colors.theme.border.divider._light}', _dark: '{colors.theme.border.divider._dark}' } },
      strong: { value: { _light: '{colors.theme.border.strong._light}', _dark: '{colors.theme.border.strong._dark}' } },
      error: { value: { _light: '{colors.feedback.error.fg}', _dark: '{colors.feedback.error.fg}' } },
    },
    icon: {
      primary: { value: { _light: '{colors.theme.icon.primary._light}', _dark: '{colors.theme.icon.primary._dark}' } },
      secondary: { value: { _light: '{colors.theme.icon.secondary._light}', _dark: '{colors.theme.icon.secondary._dark}' } },
    },

    // ELEMENTS
    header: {
      sticky: {
        bg: aliasColor('headerBar'),
      },
    },
    card: {
      border: aliasColor('hairline'),
    },
    address: {
      highlighted: {
        bg: { value: { _light: '{colors.blue.50}', _dark: '{colors.blue.900}' } },
        border: { value: { _light: '{colors.blue.200}', _dark: '{colors.blue.600}' } },
      },
    },

    // COMPONENTS
    button: {
      solid: {
        bg: {
          DEFAULT: { value: { _light: '{colors.theme.button.primary._light}', _dark: '{colors.theme.button.primary._dark}' } },
          hover: { value: { _light: '{colors.theme.button.primary.hover._light}', _dark: '{colors.theme.button.primary.hover._dark}' } },
        },
        text: {
          DEFAULT: { value: { _light: '{colors.theme.button.primary.text._light}', _dark: '{colors.theme.button.primary.text._dark}' } },
        },
      },
      outline: {
        fg: {
          DEFAULT: aliasColor('text.primary'),
        },
      },
      subtle: {
        fg: {
          DEFAULT: aliasColor('text.primary'),
        },
        bg: {
          DEFAULT: aliasColor('hairline'),
        },
      },
      dropdown: {
        fg: {
          DEFAULT: aliasColor('text.primary'),
        },
        border: {
          DEFAULT: { value: { _light: '{colors.border.strong}', _dark: '{colors.border.strong}' } },
        },
      },
      header: {
        fg: {
          DEFAULT: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.gray.400}' } },
          selected: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.whiteAlpha.800}' } },
          highlighted: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.whiteAlpha.800}' } },
        },
        bg: {
          selected: { value: { _light: '{colors.blackAlpha.50}', _dark: '{colors.whiteAlpha.100}' } },
          highlighted: { value: { _light: '{colors.orange.100}', _dark: '{colors.orange.900}' } },
        },
        border: {
          DEFAULT: { value: { _light: '{colors.border.strong}', _dark: '{colors.border.strong}' } },
        },
      },
      segmented: {
        fg: {
          DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
        },
      },
      icon_background: {
        bg: {
          DEFAULT: { value: { _light: '{colors.gray.50}', _dark: '{colors.whiteAlpha.50}' } },
        },
      },
      pagination: {
        fg: {
          DEFAULT: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.gray.50}' } },
        },
        border: {
          DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.whiteAlpha.100}' } },
        },
      },
      hero: {
        bg: {
          DEFAULT: {
            value: {
              _light: heroBannerButton?._default?.background?.[0] || '{colors.theme.button.primary._light}',
              _dark: heroBannerButton?._default?.background?.[1] || heroBannerButton?._default?.background?.[0] || '{colors.theme.button.primary._dark}',
            },
          },
          hover: {
            value: {
              _light: heroBannerButton?._hover?.background?.[0] || '{colors.hover}',
              _dark: heroBannerButton?._hover?.background?.[1] || heroBannerButton?._hover?.background?.[0] || '{colors.hover}',
            },
          },
          selected: {
            value: {
              _light: heroBannerButton?._selected?.background?.[0] || '{colors.blue.50}',
              _dark: heroBannerButton?._selected?.background?.[1] || heroBannerButton?._selected?.background?.[0] || '{colors.blue.50}',
            },
          },
        },
        fg: {
          DEFAULT: {
            value: {
              _light: heroBannerButton?._default?.text_color?.[0] || '{colors.white}',
              _dark: heroBannerButton?._default?.text_color?.[1] || heroBannerButton?._default?.text_color?.[0] || '{colors.white}',
            },
          },
          hover: {
            value: {
              _light: heroBannerButton?._hover?.text_color?.[0] || '{colors.white}',
              _dark: heroBannerButton?._hover?.text_color?.[1] || heroBannerButton?._hover?.text_color?.[0] || '{colors.white}',
            },
          },
          selected: {
            value: {
              _light: heroBannerButton?._selected?.text_color?.[0] || '{colors.blackAlpha.800}',
              _dark: heroBannerButton?._selected?.text_color?.[1] || heroBannerButton?._selected?.text_color?.[0] || '{colors.blackAlpha.800}',
            },
          },
        },
      },
    },
    closeButton: {
      fg: {
        DEFAULT: { value: { _light: '{colors.blackAlpha.500}', _dark: '{colors.whiteAlpha.500}' } },
      },
    },
    link: {
      primary: {
        DEFAULT: { value: { _light: '{colors.theme.link.primary._light}', _dark: '{colors.theme.link.primary._dark}' } },
        hover: { value: { _light: '{colors.hover}', _dark: '{colors.hover}' } },
      },
      secondary: {
        DEFAULT: { value: { _light: '{colors.text.secondary}', _dark: '{colors.text.secondary}' } },
      },
      underlaid: {
        bg: { value: { _light: '{colors.accent.soft}', _dark: '{colors.accent.soft}' } },
      },
      subtle: {
        DEFAULT: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.gray.400}' } },
        hover: { value: { _light: '{colors.blackAlpha.800}', _dark: '{colors.gray.400}' } },
      },
      navigation: {
        fg: {
          DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
          selected: { value: { _light: '{colors.theme.navigation.text.selected._light}', _dark: '{colors.theme.navigation.text.selected._dark}' } },
          hover: aliasColor('hover'),
          active: aliasColor('hover'),
        },
        bg: {
          selected: { value: { _light: '{colors.theme.navigation.bg.selected._light}', _dark: '{colors.theme.navigation.bg.selected._dark}' } },
          group: { value: { _light: '{colors.white}', _dark: '{colors.black}' } },
        },
      },
      menu: {
        DEFAULT: aliasColor('text.primary'),
      },
    },
    tooltip: {
      DEFAULT: {
        bg: { value: { _light: '{colors.gray.900}', _dark: '{colors.gray.900}' } },
        fg: { value: { _light: '{colors.white}', _dark: '{colors.white}' } },
      },
    },
    popover: {
      DEFAULT: {
        bg: { value: { _light: '{colors.bg.overlay}', _dark: '{colors.bg.overlay}' } },
        shadow: { value: { _light: '{colors.blackAlpha.200}', _dark: '{colors.whiteAlpha.300}' } },
      },
    },
    progress: {
      track: {
        DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.whiteAlpha.100}' } },
      },
    },
    progressCircle: {
      track: {
        DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.whiteAlpha.100}' } },
      },
    },
    skeleton: {
      bg: {
        start: { value: { _light: '{colors.blackAlpha.50}', _dark: '{colors.whiteAlpha.50}' } },
        end: { value: { _light: '{colors.blackAlpha.100}', _dark: '{colors.whiteAlpha.100}' } },
      },
    },
    tabs: {
      solid: {
        fg: {
          DEFAULT: { value: { _light: '{colors.theme.tabs.text.primary._light}', _dark: '{colors.theme.tabs.text.primary._dark}' } },
        },
      },
      secondary: {
        fg: {
          DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
        },
        border: {
          DEFAULT: { value: { _light: '{colors.border.strong}', _dark: '{colors.border.strong}' } },
        },
      },
      segmented: {
        fg: {
          DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
        },
      },
    },
    'switch': {
      primary: {
        bg: {
          DEFAULT: { value: { _light: '{colors.gray.300}', _dark: '{colors.whiteAlpha.400}' } },
        },
      },
    },
    alert: {
      fg: {
        DEFAULT: aliasColor('text.primary'),
      },
      bg: {
        info: { value: { _light: '{colors.blackAlpha.50}', _dark: '{colors.whiteAlpha.100}' } },
        warning: { value: { _light: '{colors.orange.100}', _dark: '{colors.orange.800/44}' } },
        warning_table: { value: { _light: '{colors.orange.50}', _dark: '{colors.orange.800/44}' } },
        success: { value: { _light: '{colors.green.100}', _dark: '{colors.green.900}' } },
        error: { value: { _light: '{colors.red.100}', _dark: '{colors.red.900}' } },
      },
    },
    toast: {
      fg: {
        DEFAULT: { value: { _light: '{colors.alert.fg}', _dark: '{colors.alert.fg}' } },
      },
      bg: {
        DEFAULT: { value: { _light: '{colors.alert.bg.info}', _dark: '{colors.alert.bg.info}' } },
        info: { value: { _light: '{colors.blue.100}', _dark: '{colors.blue.900}' } },
        warning: { value: { _light: '{colors.alert.bg.warning}', _dark: '{colors.alert.bg.warning}' } },
        success: { value: { _light: '{colors.alert.bg.success}', _dark: '{colors.alert.bg.success}' } },
        error: { value: { _light: '{colors.alert.bg.error}', _dark: '{colors.alert.bg.error}' } },
        loading: { value: { _light: '{colors.blue.100}', _dark: '{colors.blue.900}' } },
      },
    },
    input: {
      fg: {
        DEFAULT: { value: { _light: '{colors.gray.800}', _dark: '{colors.gray.50}' } },
        error: { value: { _light: '{colors.text.error}', _dark: '{colors.text.error}' } },
      },
      bg: {
        DEFAULT: { value: { _light: '{colors.bg.surface}', _dark: '{colors.bg.surface}' } },
        readOnly: { value: { _light: '{colors.bg.sunken}', _dark: '{colors.bg.sunken}' } },
      },
      border: {
        DEFAULT: { value: { _light: '{colors.border.divider}', _dark: '{colors.border.divider}' } },
        hover: { value: { _light: '{colors.border.strong}', _dark: '{colors.border.strong}' } },
        focus: { value: { _light: '{colors.accent.strong}', _dark: '{colors.accent.strong}' } },
        filled: { value: { _light: '{colors.border.strong}', _dark: '{colors.border.strong}' } },
        readOnly: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.800}' } },
        error: { value: { _light: '{colors.feedback.error.fg}', _dark: '{colors.feedback.error.fg}' } },
      },
      placeholder: {
        DEFAULT: { value: { _light: '{colors.gray.500}', _dark: '{colors.gray.500}' } },
        error: { value: { _light: '{colors.red.500}', _dark: '{colors.red.500}' } },
      },
      element: {
        DEFAULT: { value: { _light: '{colors.gray.500}', _dark: '{colors.gray.500}' } },
      },
    },
    field: {
      placeholder: {
        DEFAULT: { value: { _light: '{colors.gray.500}', _dark: '{colors.gray.500}' } },
        disabled: { value: { _light: '{colors.gray.500/20}', _dark: '{colors.gray.500/20}' } },
        error: { value: { _light: '{colors.red.500}', _dark: '{colors.red.500}' } },
      },
    },
    dialog: {
      bg: {
        DEFAULT: { value: { _light: '{colors.bg.overlay}', _dark: '{colors.bg.overlay}' } },
      },
      fg: {
        DEFAULT: { value: { _light: '{colors.text.primary}', _dark: '{colors.text.primary}' } },
      },
    },
    drawer: {
      bg: {
        DEFAULT: { value: { _light: '{colors.bg.overlay}', _dark: '{colors.bg.overlay}' } },
      },
    },
    select: {
      trigger: {
        outline: {
          fg: aliasColor('text.secondary'),
        },
      },
      indicator: {
        fg: {
          DEFAULT: { value: { _light: '{colors.gray.500}', _dark: '{colors.gray.500}' } },
        },
      },
      placeholder: {
        fg: {
          DEFAULT: { value: { _light: '{colors.gray.500}', _dark: '{colors.gray.500}' } },
          error: { value: { _light: '{colors.red.500}', _dark: '{colors.red.500}' } },
        },
      },
    },
    spinner: {
      track: {
        DEFAULT: aliasColor('hairline'),
      },
    },
    badge: {
      gray: {
        bg: aliasColor('surface'),
        fg: aliasColor('text.secondary'),
      },
      green: {
        bg: { value: { _light: '{colors.feedback.success.bg}', _dark: '{colors.feedback.success.bg}' } },
        fg: { value: { _light: '{colors.feedback.success.fg}', _dark: '{colors.feedback.success.fg}' } },
      },
      red: {
        bg: { value: { _light: '{colors.feedback.error.bg}', _dark: '{colors.feedback.error.bg}' } },
        fg: { value: { _light: '{colors.feedback.error.fg}', _dark: '{colors.feedback.error.fg}' } },
      },
      purple: {
        bg: { value: { _light: '{colors.datavis.purple.lowest}', _dark: '{colors.datavis.purple.highest}' } },
        fg: { value: { _light: '{colors.datavis.purple.high}', _dark: '{colors.datavis.purple.low}' } },
      },
      purple_alt: {
        bg: { value: { _light: '{colors.purple.100}', _dark: '{colors.purple.800}' } },
        fg: aliasColor('text.secondary'),
      },
      orange: {
        bg: { value: { _light: '{colors.feedback.warning.bg}', _dark: '{colors.feedback.warning.bg}' } },
        fg: { value: { _light: '{colors.feedback.warning.fg}', _dark: '{colors.feedback.warning.fg}' } },
      },
      blue: {
        bg: { value: { _light: '{colors.accent.soft}', _dark: '{colors.accent.soft}' } },
        fg: aliasColor('accent'),
      },
      blue_alt: {
        bg: { value: { _light: '{colors.accent.soft}', _dark: '{colors.accent.soft}' } },
        fg: aliasColor('text.secondary'),
      },
      yellow: {
        bg: aliasColor('warningSoft'),
        fg: aliasColor('warning'),
      },
      teal: {
        bg: { value: { _light: '{colors.teal.50}', _dark: '{colors.teal.800}' } },
        fg: { value: { _light: '{colors.teal.500}', _dark: '{colors.teal.100}' } },
      },
      cyan: {
        bg: { value: { _light: '{colors.cyan.50}', _dark: '{colors.cyan.800}' } },
        fg: { value: { _light: '{colors.cyan.500}', _dark: '{colors.cyan.100}' } },
      },
      pink: {
        bg: { value: { _light: '{colors.datavis.pink.lowest}', _dark: '{colors.datavis.pink.highest}' } },
        fg: { value: { _light: '{colors.datavis.pink.highest}', _dark: '{colors.datavis.pink.low}' } },
      },
      // bright badges mainly used in other projects (e.g. autoscout, dev portal, etc.)
      bright: {
        gray: {
          bg: aliasColor('surface'),
          fg: { value: { _light: '{colors.gray.600}', _dark: '{colors.gray.200}' } },
        },
        green: {
          bg: { value: { _light: '{colors.green.100}', _dark: '{colors.green.800}' } },
          fg: { value: { _light: '{colors.green.600}', _dark: '{colors.green.200}' } },
        },
        red: {
          bg: { value: { _light: '{colors.red.100}', _dark: '{colors.red.800}' } },
          fg: { value: { _light: '{colors.red.600}', _dark: '{colors.red.200}' } },
        },
        blue: {
          bg: { value: { _light: '{colors.blue.100}', _dark: '{colors.blue.800}' } },
          fg: { value: { _light: '{colors.blue.600}', _dark: '{colors.blue.200}' } },
        },
        yellow: {
          bg: { value: { _light: '{colors.yellow.100}', _dark: '{colors.yellow.800}' } },
          fg: { value: { _light: '{colors.yellow.600}', _dark: '{colors.yellow.200}' } },
        },
        teal: {
          bg: { value: { _light: '{colors.teal.100}', _dark: '{colors.teal.800}' } },
          fg: { value: { _light: '{colors.teal.600}', _dark: '{colors.teal.200}' } },
        },
        cyan: {
          bg: { value: { _light: '{colors.cyan.100}', _dark: '{colors.cyan.800}' } },
          fg: { value: { _light: '{colors.cyan.600}', _dark: '{colors.cyan.200}' } },
        },
        orange: {
          bg: { value: { _light: '{colors.orange.100}', _dark: '{colors.orange.600}' } },
          fg: { value: { _light: '{colors.orange.600}', _dark: '{colors.orange.100}' } },
        },
        purple: {
          bg: { value: { _light: '{colors.purple.50}', _dark: '{colors.purple.600}' } },
          fg: { value: { _light: '{colors.purple.600}', _dark: '{colors.purple.50}' } },
        },
        pink: {
          bg: { value: { _light: '{colors.pink.50}', _dark: '{colors.pink.600}' } },
          fg: { value: { _light: '{colors.pink.600}', _dark: '{colors.pink.50}' } },
        },
      },
    },
    tag: {
      root: {
        subtle: {
          bg: aliasColor('surface'),
          fg: aliasColor('text.secondary'),
        },
        clickable: {
          bg: aliasColor('surface'),
          fg: aliasColor('text.secondary'),
        },
        filter: {
          bg: { value: { _light: '{colors.blue.50}', _dark: '{colors.whiteAlpha.200}' } },
        },
        select: {
          bg: {
            DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.gray.800}' } },
          },
          fg: { value: { _light: '{colors.gray.500}', _dark: '{colors.whiteAlpha.800}' } },
        },
      },
    },
    table: {
      header: {
        bg: { value: { _light: '{colors.theme.table.header.bg._light}', _dark: '{colors.theme.table.header.bg._dark}' } },
        fg: { value: { _light: '{colors.theme.table.header.fg._light}', _dark: '{colors.theme.table.header.fg._dark}' } },
      },
      row: {
        hover: aliasColor('raised'),
      },
    },
    checkbox: {
      control: {
        border: {
          DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.gray.700}' } },
          hover: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.500}' } },
          readOnly: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.800}' } },
        },
      },
    },
    radio: {
      control: {
        border: {
          DEFAULT: { value: { _light: '{colors.gray.100}', _dark: '{colors.gray.700}' } },
          hover: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.500}' } },
          readOnly: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.800}' } },
        },
      },
    },
    stat: {
      indicator: {
        up: aliasColor('positive'),
        down: aliasColor('negative'),
      },
    },
    rating: {
      DEFAULT: { value: { _light: '{colors.gray.200}', _dark: '{colors.gray.700}' } },
      highlighted: { value: { _light: '{colors.yellow.400}', _dark: '{colors.yellow.400}' } },
    },
  },
  shadows: {
    popover: {
      DEFAULT: { value: { _light: '{shadows.overlay}', _dark: '{shadows.dark-lg}' } },
    },
    drawer: {
      DEFAULT: { value: { _light: '{shadows.overlay}', _dark: '{shadows.dark-lg}' } },
    },
  },
  blurs: {
    headerBar: { value: '12px' },
  },
  opacity: {
    control: {
      disabled: { value: '0.2' },
    },
  },
};

export default semanticTokens;
