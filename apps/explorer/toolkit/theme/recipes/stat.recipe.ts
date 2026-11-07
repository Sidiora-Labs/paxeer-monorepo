import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  slots: [ 'root', 'label', 'helpText', 'valueUnit', 'valueText', 'indicator' ],
  base: {
    root: {
      display: 'flex',
      flexDirection: 'column',
      gap: '1',
      position: 'relative',
      flex: '1',
    },
    label: {
      display: 'inline-flex',
      gap: '1.5',
      alignItems: 'center',
      color: 'text.muted',
      textStyle: 'label',
    },
    helpText: {
      color: 'text.primary',
      textStyle: 'small',
    },
    valueUnit: {
      color: 'text.primary',
      textStyle: 'small',
      fontWeight: 'initial',
      letterSpacing: 'initial',
    },
    valueText: {
      verticalAlign: 'baseline',
      textStyle: 'statValue',
      color: 'text.primary',
      fontWeight: '600',
      letterSpacing: 'normal',
      fontFeatureSettings: 'normal',
      fontVariantNumeric: 'tabular-nums',
      display: 'inline-flex',
      gap: '1',
    },
    indicator: {
      textStyle: 'number',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      marginEnd: 0,
      '& :where(svg)': {
        w: '1em',
        h: '1em',
      },
      '&[data-type=up]': {
        color: 'stat.indicator.up',
      },
      '&[data-type=down]': {
        color: 'stat.indicator.down',
      },
    },
  },

  variants: {
    orientation: {
      horizontal: {
        root: {
          flexDirection: 'row',
          alignItems: 'center',
        },
      },
    },
    positive: {
      'true': {
        valueText: {
          color: 'stat.indicator.up',
        },
      },
      'false': {
        valueText: {
          color: 'stat.indicator.down',
        },
      },
    },
    size: {
      sm: {
        valueText: {
          textStyle: 'statValue',
        },
      },
      md: {
        valueText: {
          textStyle: 'statValue',
        },
      },
      lg: {
        valueText: {
          textStyle: 'statValue',
        },
      },
    },
    variant: {
      scan: {
        root: {
          flexDirection: 'column',
          alignItems: 'flex-start',
          gap: '1',
          bg: 'bg.surface',
          borderWidth: '1px',
          borderStyle: 'solid',
          borderColor: 'card.border',
          borderRadius: 'md',
          boxShadow: 'card',
          px: '4',
          py: '3',
        },
        label: {
          color: 'text.muted',
          textStyle: 'xs',
          fontWeight: '500',
          letterSpacing: 'wide',
          textTransform: 'uppercase',
        },
        valueText: {
          color: 'text.primary',
          alignItems: 'baseline',
          flexWrap: 'wrap',
          '& [data-secondary]': {
            color: 'text.muted',
            textStyle: 'sm',
            fontWeight: '500',
          },
          '& [data-delta=up]': {
            color: 'stat.indicator.up',
            textStyle: 'sm',
            fontWeight: '600',
          },
          '& [data-delta=down]': {
            color: 'stat.indicator.down',
            textStyle: 'sm',
            fontWeight: '600',
          },
        },
      },
    },
  },

  compoundVariants: [
    {
      variant: 'scan',
      css: {
        root: { borderRadius: 'card', borderColor: 'hairline', p: '16px', gap: '2' },
        label: { textStyle: 'label', fontWeight: '600', letterSpacing: '0.08em' },
        valueText: {
          textStyle: 'statValue',
          fontWeight: '600',
          '& [data-secondary]': { textStyle: 'small' },
          '& [data-delta=up]': { textStyle: 'number', fontWeight: '500' },
          '& [data-delta=down]': { textStyle: 'number', fontWeight: '500' },
        },
      },
    },
  ],
  defaultVariants: {
    size: 'sm',
    orientation: 'horizontal',
  },
});
