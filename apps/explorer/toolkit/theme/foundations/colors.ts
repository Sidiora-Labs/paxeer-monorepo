import { defaultsDeep } from 'es-toolkit/compat';

import config from 'configs/app';

// Decision 2587: the explorer palette, with compatibility names for existing consumers.
const LIGHT_ROLES = {
  canvas: '#FFFFFF',
  surface: '#F7F8FA',
  raised: '#FFFFFF',
  hairline: 'rgba(16,18,24,0.08)',
  borderStrong: 'rgba(16,18,24,0.16)',
  textPrimary: '#0E1016',
  textSecondary: '#4B5262',
  textMuted: '#8A91A0',
  accent: '#2B4FDA',
  accentSoft: 'rgba(43,79,218,0.10)',
  onAccent: '#FFFFFF',
  positive: '#0E8F55',
  positiveSoft: 'rgba(14,143,85,0.10)',
  negative: '#D1343B',
  negativeSoft: 'rgba(209,52,59,0.10)',
  warning: '#B46A00',
  warningSoft: 'rgba(180,106,0,0.10)',
  headerBar: 'rgba(255,255,255,0.85)',
  chartAreaStart: 'rgba(43,79,218,0.30)',
  chartAreaStop: 'rgba(43,79,218,0)',
};

const DARK_ROLES = {
  canvas: '#0B0C10',
  surface: '#111319',
  raised: '#171A21',
  hairline: 'rgba(255,255,255,0.08)',
  borderStrong: 'rgba(255,255,255,0.16)',
  textPrimary: '#F2F4F8',
  textSecondary: '#A0A8B8',
  textMuted: '#6B7385',
  accent: '#4F7DFF',
  accentSoft: 'rgba(79,125,255,0.14)',
  onAccent: '#FFFFFF',
  positive: '#2FD27D',
  positiveSoft: 'rgba(47,210,125,0.14)',
  negative: '#FF5C5C',
  negativeSoft: 'rgba(255,92,92,0.14)',
  warning: '#F5B544',
  warningSoft: 'rgba(245,181,68,0.14)',
  headerBar: 'rgba(11,12,16,0.85)',
  chartAreaStart: 'rgba(79,125,255,0.30)',
  chartAreaStop: 'rgba(79,125,255,0)',
};

function withLegacyRoles(roles: typeof LIGHT_ROLES) {
  return {
    ...roles,
    surfaceContainer: roles.canvas,
    surfaceContainerHigh: roles.raised,
    onSurface: roles.textPrimary,
    onSurfaceVariant: roles.textSecondary,
    iconSecondary: roles.textMuted,
    outline: roles.hairline,
    dataTableBorder: roles.hairline,
    primary: roles.accent,
    onPrimary: roles.onAccent,
    primaryHover: roles.accent,
    tonal: roles.accentSoft,
    accentStrong: roles.accent,
    selectedOption: roles.accentSoft,
    graphStart: roles.chartAreaStart,
    graphStop: roles.chartAreaStop,
    success: roles.positive,
    successSoft: roles.positiveSoft,
    destructive: roles.negative,
    destructiveSoft: roles.negativeSoft,
    focusRing: roles.accent,
    sparklineUp: roles.positive,
    sparklineDown: roles.negative,
    chartGrid: roles.hairline,
  };
}

export const LIGHT = withLegacyRoles(LIGHT_ROLES);
export const DARK = withLegacyRoles(DARK_ROLES);

function paletteRole(role: keyof typeof LIGHT) {
  return { _light: { value: LIGHT[role] }, _dark: { value: DARK[role] } };
}

const DEFAULT_THEME_COLORS = {
  canvas: paletteRole('canvas'),
  surface: paletteRole('surface'),
  raised: paletteRole('raised'),
  hairline: paletteRole('hairline'),
  borderStrong: paletteRole('borderStrong'),
  onAccent: paletteRole('onAccent'),
  positive: paletteRole('positive'),
  positiveSoft: paletteRole('positiveSoft'),
  negative: paletteRole('negative'),
  negativeSoft: paletteRole('negativeSoft'),
  warning: paletteRole('warning'),
  warningSoft: paletteRole('warningSoft'),
  headerBar: paletteRole('headerBar'),
  focusRing: paletteRole('focusRing'),
  sparklineUp: paletteRole('sparklineUp'),
  sparklineDown: paletteRole('sparklineDown'),
  chartGrid: paletteRole('chartGrid'),
  chartAreaStart: paletteRole('chartAreaStart'),
  chartAreaStop: paletteRole('chartAreaStop'),
  graphStart: paletteRole('graphStart'),
  graphStop: paletteRole('graphStop'),
  success: paletteRole('success'),
  successSoft: paletteRole('successSoft'),
  destructive: paletteRole('destructive'),
  destructiveSoft: paletteRole('destructiveSoft'),
  tonal: paletteRole('tonal'),
  outline: paletteRole('outline'),
  dataTableBorder: paletteRole('dataTableBorder'),
  selectedOption: paletteRole('selectedOption'),
  bg: {
    primary: {
      _light: { value: LIGHT.surfaceContainer },
      _dark: { value: DARK.surfaceContainer },
    },
    surface: {
      _light: { value: LIGHT.surface },
      _dark: { value: DARK.surface },
    },
    sunken: {
      _light: { value: LIGHT.surfaceContainerHigh },
      _dark: { value: DARK.surfaceContainerHigh },
    },
    overlay: {
      _light: { value: LIGHT.raised },
      _dark: { value: DARK.surfaceContainerHigh },
    },
  },
  text: {
    primary: {
      _light: { value: LIGHT.onSurface },
      _dark: { value: DARK.onSurface },
    },
    secondary: {
      _light: { value: LIGHT.onSurfaceVariant },
      _dark: { value: DARK.onSurfaceVariant },
    },
    muted: {
      _light: { value: LIGHT.textMuted },
      _dark: { value: DARK.textMuted },
    },
  },
  border: {
    divider: {
      _light: { value: LIGHT.outline },
      _dark: { value: DARK.outline },
    },
    strong: {
      _light: { value: LIGHT.borderStrong },
      _dark: { value: DARK.borderStrong },
    },
  },
  accent: {
    primary: {
      _light: { value: LIGHT.accent },
      _dark: { value: DARK.accent },
    },
    strong: {
      _light: { value: LIGHT.accentStrong },
      _dark: { value: DARK.accentStrong },
    },
    soft: {
      _light: { value: LIGHT.accentSoft },
      _dark: { value: DARK.accentSoft },
    },
  },
  hover: {
    _light: { value: LIGHT.accentStrong },
    _dark: { value: DARK.accentStrong },
  },
  selected: {
    control: {
      text: {
        _light: { value: LIGHT.accent },
        _dark: { value: DARK.accent },
      },
      bg: {
        _light: { value: LIGHT.accentSoft },
        _dark: { value: DARK.accentSoft },
      },
    },
    option: {
      bg: {
        _light: { value: LIGHT.selectedOption },
        _dark: { value: DARK.selectedOption },
      },
    },
  },
  icon: {
    primary: {
      _light: { value: LIGHT.onSurfaceVariant },
      _dark: { value: DARK.onSurfaceVariant },
    },
    secondary: {
      _light: { value: LIGHT.iconSecondary },
      _dark: { value: DARK.iconSecondary },
    },
  },
  button: {
    primary: {
      _light: { value: LIGHT.primary },
      _dark: { value: DARK.primary },
      text: {
        _light: { value: LIGHT.onPrimary },
        _dark: { value: DARK.onPrimary },
      },
      hover: {
        _light: { value: LIGHT.primaryHover },
        _dark: { value: DARK.primaryHover },
      },
    },
  },
  link: {
    primary: {
      _light: { value: LIGHT.accent },
      _dark: { value: DARK.accent },
    },
  },
  graph: {
    line: {
      _light: { value: LIGHT.accent },
      _dark: { value: DARK.accent },
    },
    gradient: {
      start: {
        _light: { value: LIGHT.graphStart },
        _dark: { value: DARK.graphStart },
      },
      stop: {
        _light: { value: LIGHT.graphStop },
        _dark: { value: DARK.graphStop },
      },
    },
  },
  navigation: {
    bg: {
      selected: {
        _light: { value: LIGHT.surfaceContainerHigh },
        _dark: { value: DARK.surfaceContainerHigh },
      },
    },
    text: {
      selected: {
        _light: { value: LIGHT.onSurface },
        _dark: { value: DARK.onSurface },
      },
    },
  },
  stats: {
    bg: {
      _light: { value: LIGHT.surface },
      _dark: { value: DARK.surface },
    },
  },
  topbar: {
    bg: {
      _light: { value: LIGHT.headerBar },
      _dark: { value: DARK.headerBar },
    },
  },
  tabs: {
    text: {
      primary: {
        _light: { value: LIGHT.onSurface },
        _dark: { value: DARK.onSurface },
      },
    },
  },
  table: {
    header: {
      bg: {
        _light: { value: LIGHT.surfaceContainer },
        _dark: { value: DARK.canvas },
      },
      fg: {
        _light: { value: LIGHT.textMuted },
        _dark: { value: DARK.textMuted },
      },
    },
  },
  feedback: {
    success: {
      fg: {
        _light: { value: LIGHT.success },
        _dark: { value: DARK.success },
      },
      bg: {
        _light: { value: LIGHT.successSoft },
        _dark: { value: DARK.successSoft },
      },
    },
    error: {
      fg: {
        _light: { value: LIGHT.destructive },
        _dark: { value: DARK.destructive },
      },
      bg: {
        _light: { value: LIGHT.destructiveSoft },
        _dark: { value: DARK.destructiveSoft },
      },
    },
    warning: {
      fg: {
        _light: { value: LIGHT.warning },
        _dark: { value: DARK.warning },
      },
      bg: {
        _light: { value: LIGHT.warningSoft },
        _dark: { value: DARK.warningSoft },
      },
    },
  },
};

const colors = {
  // BASE COLORS
  green: {
    '50': { value: '#F0FFF4' },
    '100': { value: '#C6F6D5' },
    '200': { value: '#9AE6B4' },
    '300': { value: '#68D391' },
    '400': { value: '#48BB78' },
    '500': { value: '#38A169' },
    '600': { value: '#25855A' },
    '700': { value: '#276749' },
    '800': { value: '#22543D' },
    '900': { value: '#1C4532' },
  },
  blue: {
    '50': { value: '#EBF8FF' },
    '100': { value: '#BEE3F8' },
    '200': { value: '#90CDF4' },
    '300': { value: '#63B3ED' },
    '400': { value: '#4299E1' },
    '500': { value: '#3182CE' },
    '600': { value: '#2B6CB0' },
    '700': { value: '#2C5282' },
    '800': { value: '#2A4365' },
    '900': { value: '#1A365D' },
  },
  red: {
    '50': { value: '#FFF5F5' },
    '100': { value: '#FED7D7' },
    '200': { value: '#FEB2B2' },
    '300': { value: '#FC8181' },
    '400': { value: '#F56565' },
    '500': { value: '#E53E3E' },
    '600': { value: '#C53030' },
    '700': { value: '#9B2C2C' },
    '800': { value: '#822727' },
    '900': { value: '#63171B' },
  },
  orange: {
    '50': { value: '#FFFAF0' },
    '100': { value: '#FEEBCB' },
    '200': { value: '#FBD38D' },
    '300': { value: '#F6AD55' },
    '400': { value: '#ED8936' },
    '500': { value: '#DD6B20' },
    '600': { value: '#C05621' },
    '700': { value: '#9C4221' },
    '800': { value: '#7B341E' },
    '900': { value: '#652B19' },
  },
  yellow: {
    '50': { value: '#FFFFF0' },
    '100': { value: '#FEFCBF' },
    '200': { value: '#FAF089' },
    '300': { value: '#F6E05E' },
    '400': { value: '#ECC94B' },
    '500': { value: '#D69E2E' },
    '600': { value: '#B7791F' },
    '700': { value: '#975A16' },
    '800': { value: '#744210' },
    '900': { value: '#5F370E' },
  },
  gray: {
    '50': { value: '#F8F9FC' },
    '100': { value: '#EFF2F7' },
    '200': { value: '#E1E6EC' },
    '300': { value: '#CDD4DC' },
    '400': { value: '#B2BBC5' },
    '500': { value: '#80868B' },
    '600': { value: '#45474D' },
    '700': { value: '#2F3034' },
    '800': { value: '#212226' },
    '900': { value: '#18191D' },
  },
  teal: {
    '50': { value: '#E6FFFA' },
    '100': { value: '#B2F5EA' },
    '200': { value: '#81E6D9' },
    '300': { value: '#4FD1C5' },
    '400': { value: '#38B2AC' },
    '500': { value: '#319795' },
    '600': { value: '#2C7A7B' },
    '700': { value: '#285E61' },
    '800': { value: '#234E52' },
    '900': { value: '#1D4044' },
  },
  cyan: {
    '50': { value: '#EDFDFD' },
    '100': { value: '#C4F1F9' },
    '200': { value: '#9DECF9' },
    '300': { value: '#76E4F7' },
    '400': { value: '#0BC5EA' },
    '500': { value: '#00B5D8' },
    '600': { value: '#00A3C4' },
    '700': { value: '#0987A0' },
    '800': { value: '#086F83' },
    '900': { value: '#065666' },
  },
  purple: {
    '50': { value: '#FAF5FF' },
    '100': { value: '#E9D8FD' },
    '200': { value: '#D6BCFA' },
    '300': { value: '#B794F4' },
    '400': { value: '#9F7AEA' },
    '500': { value: '#805AD5' },
    '600': { value: '#6B46C1' },
    '700': { value: '#553C9A' },
    '800': { value: '#44337A' },
    '900': { value: '#322659' },
  },
  pink: {
    '50': { value: '#FFF5F7' },
    '100': { value: '#FED7E2' },
    '200': { value: '#FBB6CE' },
    '300': { value: '#F687B3' },
    '400': { value: '#ED64A6' },
    '500': { value: '#D53F8C' },
    '600': { value: '#B83280' },
    '700': { value: '#97266D' },
    '800': { value: '#702459' },
    '900': { value: '#521B41' },
  },
  black: { value: '#121317' },
  white: { value: '#ffffff' },
  whiteAlpha: {
    '50': { value: 'RGBA(255, 255, 255, 0.04)' },
    '100': { value: 'RGBA(255, 255, 255, 0.06)' },
    '200': { value: 'RGBA(255, 255, 255, 0.08)' },
    '300': { value: 'RGBA(255, 255, 255, 0.16)' },
    '400': { value: 'RGBA(255, 255, 255, 0.24)' },
    '500': { value: 'RGBA(255, 255, 255, 0.36)' },
    '600': { value: 'RGBA(255, 255, 255, 0.48)' },
    '700': { value: 'RGBA(255, 255, 255, 0.64)' },
    '800': { value: 'RGBA(255, 255, 255, 0.80)' },
    '900': { value: 'RGBA(255, 255, 255, 0.92)' },
  },
  blackAlpha: {
    '50': { value: 'RGBA(33, 34, 38, 0.04)' },
    '100': { value: 'RGBA(33, 34, 38, 0.06)' },
    '200': { value: 'RGBA(33, 34, 38, 0.08)' },
    '300': { value: 'RGBA(33, 34, 38, 0.16)' },
    '400': { value: 'RGBA(33, 34, 38, 0.24)' },
    '500': { value: 'RGBA(33, 34, 38, 0.36)' },
    '600': { value: 'RGBA(33, 34, 38, 0.48)' },
    '700': { value: 'RGBA(33, 34, 38, 0.64)' },
    '800': { value: 'RGBA(33, 34, 38, 0.80)' },
    '900': { value: 'RGBA(33, 34, 38, 0.92)' },
  },

  datavis: {
    blue: {
      lowest: { value: '#DCF1FF' },
      low: { value: '#9DD2FF' },
      mid: { value: '#3C90FF' },
      high: { value: '#2B4FDA' },
      highest: { value: '#1E2867' },
    },
    green: {
      lowest: { value: '#DAF9D4' },
      low: { value: '#9BE69A' },
      mid: { value: '#0EBC5F' },
      high: { value: '#008052' },
      highest: { value: '#005143' },
    },
    grey: {
      lowest: { value: '#EFF2F7' },
      low: { value: '#E1E6EC' },
      mid: { value: '#B2BBC5' },
      high: { value: '#45474D' },
      highest: { value: '#212226' },
    },
    pink: {
      lowest: { value: '#FFDBF5' },
      low: { value: '#FFB5E8' },
      mid: { value: '#FF88D3' },
      high: { value: '#DE249E' },
      highest: { value: '#72004A' },
    },
    purple: {
      lowest: { value: '#E0E7FF' },
      low: { value: '#B8C0FF' },
      mid: { value: '#7372FE' },
      high: { value: '#4D34CF' },
      highest: { value: '#2E1C6D' },
    },
    red: {
      lowest: { value: '#FFEEF7' },
      low: { value: '#FFACC2' },
      mid: { value: '#FF4C45' },
      high: { value: '#BE1C1B' },
      highest: { value: '#7D0304' },
    },
    yellow: {
      lowest: { value: '#FCFFAD' },
      low: { value: '#FFF350' },
      mid: { value: '#FFCF03' },
      high: { value: '#F29900' },
      highest: { value: '#D05600' },
    },
  },

  // BRAND COLORS
  github: { value: '#171923' },
  telegram: { value: '#2775CA' },
  linkedin: { value: '#1564BA' },
  discord: { value: '#9747FF' },
  slack: { value: '#1BA27A' },
  twitter: { value: '#000000' },
  opensea: { value: '#2081E2' },
  facebook: { value: '#4460A0' },
  medium: { value: '#231F20' },
  reddit: { value: '#FF4500' },
  celo: { value: '#FCFF52' },
  clusters: { value: '#DE6061' },

  // THEME COLORS
  theme: defaultsDeep(config.UI.colorTheme.overrides, DEFAULT_THEME_COLORS),
};

export default colors;
