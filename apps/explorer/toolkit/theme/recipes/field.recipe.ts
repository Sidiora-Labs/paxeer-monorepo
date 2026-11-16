import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  className: 'chakra-field',
  slots: [ 'root', 'label', 'requiredIndicator', 'errorText', 'helperText' ],
  base: {
    requiredIndicator: {
      color: 'inherit',
      lineHeight: 'inherit',
    },
    root: {
      display: 'flex',
      width: '100%',
      position: 'relative',
      gap: '1',
    },
    label: {
      _motionReduce: { transitionProperty: 'color, background-color, border-color, opacity' },
      display: 'flex',
      alignItems: 'center',
      textAlign: 'start',
      textStyle: 'sm',
      fontWeight: 'medium',
      gap: '0',
      userSelect: 'none',
      zIndex: '1',
      _disabled: {
        opacity: 'control.disabled',
      },
      _invalid: {
        color: 'input.fg.error',
      },
    },
    errorText: {
      display: 'inline-flex',
      alignItems: 'center',
      fontWeight: 'medium',
      gap: '1',
      color: 'input.fg.error',
      textStyle: 'sm',
    },
    helperText: {
      color: 'fg.muted',
      textStyle: 'sm',
    },
  },

  variants: {
    floating: {
      'true': {
        label: {
          pos: 'absolute',
          bg: 'bg.primary',
          top: 'var(--chakra-spacing-0\\.5)',
          left: 'var(--chakra-spacing-0\\.5)',
          color: 'input.placeholder',
          width: 'calc(100% - var(--chakra-spacing-1))',
          borderRadius: 'base',
          pointerEvents: 'none',
          transformOrigin: 'top left',
          transitionProperty: 'font-size, line-height, padding, background-color',
          transitionDuration: 'fast',
          transitionTimingFunction: 'standard',
        },
      },
    },
    size: {
      sm: {
        label: {
          fontSize: 'sm',
        },
      },
      md: {
        label: {
          fontSize: 'md',
        },
      },
      lg: {
        label: {
          fontSize: 'md',
        },
      },
      // special size for textarea
      '2xl': {
        label: {
          fontSize: 'md',
        },
      },
    },
    orientation: {
      vertical: {
        root: {
          flexDirection: 'column',
          alignItems: 'flex-start',
        },
      },
      horizontal: {
        root: {
          flexDirection: 'row',
          alignItems: 'center',
          justifyContent: 'space-between',
        },
        label: {
          flex: '0 0 var(--field-label-width, var(--chakra-spacing-20))',
        },
      },
    },
  },

  compoundVariants: [
    {
      size: 'lg',
      floating: true,
      css: {
        label: {
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          padding: 'var(--chakra-spacing-2\\.5) var(--chakra-spacing-4) 0 var(--chakra-spacing-4)',
          textStyle: 'xs',
          _peerPlaceholderShown: {
            padding: 'var(--chakra-spacing-4)',
            textStyle: 'md',
          },
          _peerFocusVisible: {
            padding: 'var(--chakra-spacing-2\\.5) var(--chakra-spacing-4) 0 var(--chakra-spacing-4)',
            textStyle: 'xs',
          },
          _readOnly: {
            bg: 'input.bg.readOnly',
          },
        },
        errorText: {
          fontSize: 'inherit',
          lineHeight: 'inherit',
        },
      },
    },
    {
      size: '2xl',
      floating: true,
      css: {
        label: {
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          // 20px = scrollbar width
          // 4px = border width
          width: 'calc(100% - var(--chakra-spacing-1) - var(--chakra-spacing-5))',
          padding: 'var(--chakra-spacing-4) var(--chakra-spacing-4) 0 var(--chakra-spacing-4)',
          textStyle: 'xs',
          borderTopRightRadius: '0',
          borderBottomRightRadius: '0',
          _peerPlaceholderShown: {
            textStyle: 'md',
          },
          _peerFocusVisible: {
            textStyle: 'xs',
          },
          _readOnly: {
            bg: 'input.bg.readOnly',
          },
        },
        errorText: {
          fontSize: 'inherit',
          lineHeight: 'inherit',
        },
      },
    },
  ],

  defaultVariants: {
    floating: false,
    orientation: 'vertical',
  },
});
