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

export const textStyles: ThemingConfig['textStyles'] = {
  display: {
    value: { fontSize: '40px', lineHeight: '44px', fontWeight: '600', letterSpacing: '-0.02em', fontFamily: 'body' },
  },
  h1: {
    value: { fontSize: '28px', lineHeight: '32px', fontWeight: '600', letterSpacing: '-0.01em', fontFamily: 'body' },
  },
  h2: {
    value: { fontSize: '20px', lineHeight: '24px', fontWeight: '600', fontFamily: 'body' },
  },
  body: {
    value: { fontSize: '14px', lineHeight: '20px', fontWeight: '400', fontFamily: 'body' },
  },
  small: {
    value: { fontSize: '13px', lineHeight: '18px', fontWeight: '400', fontFamily: 'body' },
  },
  label: {
    value: {
      fontSize: '11px', lineHeight: '16px', fontWeight: '600', fontFamily: 'body',
      textTransform: 'uppercase', letterSpacing: '0.08em', color: 'text.muted',
    },
  },
  number: {
    value: { fontSize: '13px', lineHeight: '18px', fontWeight: '500', fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  },
  statValue: {
    value: { fontSize: '24px', lineHeight: '28px', fontWeight: '600', fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  },
  heading: {
    xl: {
      value: {
        fontSize: '32px',
        lineHeight: '36px',
        fontWeight: '500',
        letterSpacing: '-0.01067em',
        fontFamily: 'heading',
      },
    },
    lg: {
      value: {
        fontSize: '24px',
        lineHeight: '28px',
        fontWeight: '500',
        letterSpacing: '-0.00583em',
        fontFamily: 'heading',
      },
    },
    md: {
      value: {
        fontSize: '18px',
        lineHeight: '24px',
        fontWeight: '500',
        letterSpacing: '-0.00389em',
        fontFamily: 'heading',
      },
    },
    sm: {
      value: {
        fontSize: '16px',
        lineHeight: '24px',
        fontWeight: '500',
        letterSpacing: '-0.00389em',
        fontFamily: 'heading',
      },
    },
    xs: {
      value: {
        fontSize: '14px',
        lineHeight: '20px',
        fontWeight: '500',
        letterSpacing: '0.01143em',
        fontFamily: 'heading',
      },
    },
  },
  text: {
    xl: {
      value: {
        fontSize: '20px',
        lineHeight: '28px',
        fontWeight: '400',
        letterSpacing: '-0.00444em',
        fontFamily: 'body',
      },
    },
    md: {
      value: {
        fontSize: '16px',
        lineHeight: '24px',
        fontWeight: '400',
        letterSpacing: '0.01188em',
        fontFamily: 'body',
      },
    },
    sm: {
      value: {
        fontSize: '14px',
        lineHeight: '20px',
        fontWeight: '400',
        letterSpacing: '0.01143em',
        fontFamily: 'body',
      },
    },
    xs: {
      value: {
        fontSize: '12px',
        lineHeight: '16px',
        fontWeight: '400',
        letterSpacing: '0.00917em',
        fontFamily: 'body',
      },
    },
  },
};
