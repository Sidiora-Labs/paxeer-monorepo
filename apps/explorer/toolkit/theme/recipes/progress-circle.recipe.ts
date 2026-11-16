import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  slots: [ 'root', 'circle', 'circleTrack', 'circleRange', 'label', 'valueText' ],
  base: {
    root: {
      display: 'inline-flex',
      textStyle: 'sm',
      position: 'relative',
    },
    circle: {
      _indeterminate: {
        _motionSafe: { animation: 'spin var(--chakra-durations-shimmer) linear infinite' },
      },
    },
    circleTrack: {
      '--track-color': 'colors.progressCircle.track',
      stroke: 'var(--track-color)',
    },
    circleRange: {
      _motionReduce: { transitionProperty: 'color, background-color, border-color, opacity' },
      stroke: 'selected.option.bg',
      transitionProperty: 'stroke-dasharray',
      transitionDuration: 'var(--chakra-durations-progress)',
      _indeterminate: {
        _motionSafe: { animation: 'circular-progress var(--chakra-durations-circular) linear infinite' },
      },
    },
    label: {
      display: 'inline-flex',
    },
    valueText: {
      lineHeight: '1',
      fontWeight: 'medium',
      letterSpacing: 'tight',
      fontVariantNumeric: 'tabular-nums',
    },
  },

  variants: {
    size: {
      sm: {
        circle: {
          '--size': 'var(--chakra-spacing-5)',
          '--thickness': 'var(--chakra-border-widths-2)',
        },
        valueText: {
          textStyle: 'xs',
        },
      },
      md: {
        circle: {
          '--size': 'var(--chakra-spacing-8)',
          '--thickness': 'var(--chakra-border-widths-3)',
        },
        valueText: {
          textStyle: 'xs',
        },
      },
      lg: {
        circle: {
          '--size': 'var(--chakra-spacing-20)',
          '--thickness': 'var(--chakra-border-widths-8)',
        },
        valueText: {
          textStyle: 'sm',
        },
      },
    },
  },

  defaultVariants: {
    size: 'md',
  },
});
