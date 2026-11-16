import type { SystemConfig } from '@chakra-ui/react';

import addressEntity from './globals/address-entity';
import entity from './globals/entity';
import recaptcha from './globals/recaptcha';
import scrollbar from './globals/scrollbar';

const webkitAutofillOverrides = {
  WebkitTextFillColor: 'var(--chakra-colors-input-fg)',
  WebkitBoxShadow: 'var(--chakra-shadows-autofill)',
};

const webkitAutofillRules = {
  '&:-webkit-autofill': webkitAutofillOverrides,
  '&:-webkit-autofill:hover': webkitAutofillOverrides,
  '&:-webkit-autofill:focus': webkitAutofillOverrides,
};

const globalCss: SystemConfig['globalCss'] = {
  '*, *::before, *::after': {
    boxSizing: 'border-box',
  },
  ':root': {
    colorScheme: 'light dark',
  },
  ':root.light': {
    colorScheme: 'light',
  },
  ':root.dark': {
    colorScheme: 'dark',
  },
  html: {
    minWidth: 0,
    scrollbarGutter: 'stable',
    scrollPaddingBlock: 'spacing.4',
    textSizeAdjust: '100%',
  },
  body: {
    bg: 'global.body.bg',
    color: 'global.body.fg',
    textStyle: 'body',
    fontFamily: 'body',
    fontSizeAdjust: 'from-font',
    overflowWrap: 'anywhere',
    minWidth: 0,
    WebkitTapHighlightColor: 'transparent',
    fontVariantLigatures: 'no-contextual',
    textRendering: 'optimizeLegibility',
  },
  ':where(h1, h2, h3, h4, h5, h6)': {
    textWrap: 'balance',
  },
  ':where(p, li, dd, blockquote)': {
    textWrap: 'pretty',
  },
  ':where(img, video, svg, canvas)': {
    maxWidth: '100%',
    height: 'auto',
  },
  ':where(button, input, textarea, select)': {
    font: 'inherit',
  },
  ':where(button, input:not([type=checkbox], [type=radio], [type=hidden]), select, textarea, [role=tab], [role=menuitem], [role=option], label:has(input[type=checkbox], input[type=radio]))': {
    '@media (pointer: coarse)': { minBlockSize: 'touchTarget', minInlineSize: 'touchTarget' },
  },
  ':where(input, select, textarea)': {
    '@media (pointer: coarse)': { fontSize: 'max(var(--chakra-font-sizes-text-md), 1em)' },
  },
  ':focus-visible': {
    outline: 'var(--chakra-border-widths-2) solid',
    outlineColor: 'focusRing',
    outlineOffset: 'var(--chakra-spacing-0\\.5)',
  },
  mark: {
    bg: 'global.mark.bg',
    color: 'inherit',
  },
  '::selection': {
    bg: 'global.selection.bg',
  },
  'svg *::selection': {
    color: 'none',
    background: 'none',
  },
  form: {
    w: '100%',
  },
  input: {
    // hide number input arrows in Google Chrome
    '&::-webkit-outer-spin-button, &::-webkit-inner-spin-button': {
      WebkitAppearance: 'none',
      margin: 0,
    },
    ...webkitAutofillRules,
  },
  textarea: {
    ...webkitAutofillRules,
  },
  select: {
    ...webkitAutofillRules,
  },
  ...recaptcha,
  ...scrollbar,
  ...entity,
  ...addressEntity,
};

export default globalCss;
