import { defineRecipe } from '@chakra-ui/react';

export const recipe = defineRecipe({
  base: {},

  variants: {
    loading: {
      'true': {},
      'false': {},
      // special value to override the default behavior of the skeleton in Chakra
      // it uses "background: unset" when the loading prop is set to false
      // but it causes issues with background color of child element (e.g. button, badge, etc.)
      // so, instead of the "loading" prop, we use the "state" prop to control the skeleton (see below)
      reset: {},
    },
    state: {
      loading: {
        borderRadius: 'base',
        boxShadow: 'none',
        backgroundClip: 'padding-box',
        cursor: 'default',
        color: 'transparent',
        borderWidth: 'var(--chakra-border-widths-0)',
        pointerEvents: 'none',
        userSelect: 'none',
        '& > *': {
          visibility: 'hidden',
        },
      },
    },
    variant: {
      pulse: {
        background: 'bg.emphasized',
        _motionSafe: { animation: 'pulse' },
        animationDuration: 'var(--duration, var(--chakra-durations-pulse))',
      },
      shine: {
        opacity: 0.5,
        '--animate-from': '100%',
        '--animate-to': '-100%',
        '--start-color': 'colors.skeleton.bg.start',
        '--end-color': 'colors.skeleton.bg.end',
        position: 'relative',
        overflow: 'hidden',
        backgroundColor: 'var(--start-color)',
        animation: 'none',
        '&::after': {
          content: '""',
          position: 'absolute',
          inset: '0',
          left: '0',
          width: '100%',
          pointerEvents: 'none',
          willChange: 'transform',
          transform: 'translateX(-100%)',
          backgroundImage:
            'linear-gradient(90deg, transparent 0%, transparent 32%, var(--end-color) 46%, var(--end-color) 54%, transparent 68%, transparent 100%)',
          _motionSafe: { animation: 'skeletonShimmer var(--duration, var(--chakra-durations-shimmer)) linear infinite' },
        },
        _motionReduce: {
          '&::after': {
            animation: 'none',
          },
        },
      },
      none: {
        animation: 'none',
      },
    },
  },

  defaultVariants: {
    variant: 'shine',
    loading: 'reset',
  },
});
