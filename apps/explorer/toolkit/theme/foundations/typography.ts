import type { ThemingConfig } from '@chakra-ui/react';

import type { ExcludeUndefined } from 'types/utils';

import config from 'configs/app';

export const BODY_TYPEFACE = config.UI.fonts.body?.name ?? 'Inter, InterFallback';
export const HEADING_TYPEFACE = BODY_TYPEFACE;
export const MONO_TYPEFACE = '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace';

// The product fallback stack, so a deployment keeps the same metrics while the
// configured typeface loads.
export const FALLBACK_TYPEFACES = 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif';

export const fonts: ExcludeUndefined<ThemingConfig['tokens']>['fonts'] = {
  heading: { value: `${ HEADING_TYPEFACE }, ${ FALLBACK_TYPEFACES }` },
  body: { value: `${ BODY_TYPEFACE }, ${ FALLBACK_TYPEFACES }` },
  mono: { value: MONO_TYPEFACE },
};

export const fontSizes = {
  fluidDisplay: { value: 'clamp(2rem, 1.833333rem + 0.740741vw, 2.5rem)' },
  fluidH1: { value: 'clamp(1.5rem, 1.416667rem + 0.37037vw, 1.75rem)' },
  fluidH2: { value: 'clamp(1.125rem, 1.083333rem + 0.185185vw, 1.25rem)' },
  body: { value: '0.875rem' },
  small: { value: '0.8125rem' },
  label: { value: '0.6875rem' },
  number: { value: '0.8125rem' },
  statValue: { value: '1.5rem' },
  headingXl: { value: '2rem' },
  headingLg: { value: '1.5rem' },
  headingMd: { value: '1.125rem' },
  headingSm: { value: '1rem' },
  textXl: { value: '1.25rem' },
  textMd: { value: '1rem' },
  textXs: { value: '0.75rem' },
} as const;

export const lineHeights = {
  display: { value: '2.75rem' },
  h1: { value: '2rem' },
  h2: { value: '1.5rem' },
  body: { value: '1.25rem' },
  small: { value: '1.125rem' },
  label: { value: '1rem' },
  statValue: { value: '1.75rem' },
  headingXl: { value: '2.25rem' },
} as const;

export const textStyles: ThemingConfig['textStyles'] = {
  display: {
    value: { fontSize: fontSizes.fluidDisplay.value, lineHeight: lineHeights.display.value, fontWeight: 'semibold', letterSpacing: '-0.02em', fontFamily: 'body' },
  },
  h1: {
    value: { fontSize: fontSizes.fluidH1.value, lineHeight: lineHeights.h1.value, fontWeight: 'semibold', letterSpacing: '-0.01em', fontFamily: 'body' },
  },
  h2: {
    value: { fontSize: fontSizes.fluidH2.value, lineHeight: lineHeights.h2.value, fontWeight: 'semibold', fontFamily: 'body' },
  },
  body: {
    value: { fontSize: fontSizes.body.value, lineHeight: lineHeights.body.value, fontWeight: 'normal', fontFamily: 'body' },
  },
  small: {
    value: { fontSize: fontSizes.small.value, lineHeight: lineHeights.small.value, fontWeight: 'normal', fontFamily: 'body' },
  },
  label: {
    value: {
      fontSize: fontSizes.label.value, lineHeight: lineHeights.label.value, fontWeight: 'semibold', fontFamily: 'body',
      textTransform: 'uppercase', letterSpacing: '0.08em', color: 'text.muted',
    },
  },
  number: {
    value: { fontSize: fontSizes.small.value, lineHeight: lineHeights.small.value, fontWeight: 'medium', fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  },
  statValue: {
    value: { fontSize: fontSizes.statValue.value, lineHeight: lineHeights.statValue.value, fontWeight: 'semibold', fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  },
  heading: {
    xl: {
      value: {
        fontSize: fontSizes.headingXl.value,
        lineHeight: lineHeights.headingXl.value,
        fontWeight: 'medium',
        letterSpacing: '-0.01067em',
        fontFamily: 'heading',
      },
    },
    lg: {
      value: {
        fontSize: fontSizes.headingLg.value,
        lineHeight: lineHeights.statValue.value,
        fontWeight: 'medium',
        letterSpacing: '-0.00583em',
        fontFamily: 'heading',
      },
    },
    md: {
      value: {
        fontSize: fontSizes.headingMd.value,
        lineHeight: lineHeights.h2.value,
        fontWeight: 'medium',
        letterSpacing: '-0.00389em',
        fontFamily: 'heading',
      },
    },
    sm: {
      value: {
        fontSize: fontSizes.textMd.value,
        lineHeight: lineHeights.h2.value,
        fontWeight: 'medium',
        letterSpacing: '-0.00389em',
        fontFamily: 'heading',
      },
    },
    xs: {
      value: {
        fontSize: fontSizes.body.value,
        lineHeight: lineHeights.body.value,
        fontWeight: 'medium',
        letterSpacing: '0.01143em',
        fontFamily: 'heading',
      },
    },
  },
  text: {
    xl: {
      value: {
        fontSize: fontSizes.textXl.value,
        lineHeight: lineHeights.statValue.value,
        fontWeight: 'normal',
        letterSpacing: '-0.00444em',
        fontFamily: 'body',
      },
    },
    md: {
      value: {
        fontSize: fontSizes.textMd.value,
        lineHeight: lineHeights.h2.value,
        fontWeight: 'normal',
        letterSpacing: '0.01188em',
        fontFamily: 'body',
      },
    },
    sm: {
      value: {
        fontSize: fontSizes.body.value,
        lineHeight: lineHeights.body.value,
        fontWeight: 'normal',
        letterSpacing: '0.01143em',
        fontFamily: 'body',
      },
    },
    xs: {
      value: {
        fontSize: fontSizes.textXs.value,
        lineHeight: lineHeights.label.value,
        fontWeight: 'normal',
        letterSpacing: '0.00917em',
        fontFamily: 'body',
      },
    },
  },
};
