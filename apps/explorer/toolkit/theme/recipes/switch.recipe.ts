import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  slots: [ 'root', 'label', 'indicator', 'control', 'thumb' ],
  className: 'chakra-switch',
  base: {
    root: {
      display: 'inline-flex',
      gap: '2',
      alignItems: 'center',
      position: 'relative',
      verticalAlign: 'middle',
      '--switch-diff': 'calc(var(--switch-width) - var(--switch-height))',
      '--switch-x': {
        base: 'var(--switch-diff)',
        _rtl: 'calc(var(--switch-diff) * -1)',
      },
    },

    label: {
      userSelect: 'none',
      fontWeight: 'normal',
      color: 'text.primary',
      _disabled: {
        opacity: 'control.disabled',
      },
    },

    indicator: {
      _motionReduce: { transitionProperty: 'color, background-color, border-color, opacity' },
      position: 'absolute',
      height: 'var(--switch-height)',
      width: 'var(--switch-height)',
      fontSize: 'var(--switch-indicator-font-size)',
      flexShrink: 0,
      userSelect: 'none',
      display: 'grid',
      placeContent: 'center',
      transition: 'inset-inline-start var(--chakra-durations-toggle) var(--chakra-easings-standard)',
      insetInlineStart: 'calc(var(--switch-x) - var(--chakra-spacing-0\\.5))',
      _checked: {
        insetInlineStart: 'var(--chakra-spacing-0\\.5)',
      },
    },

    control: {
      display: 'inline-flex',
      gap: 'var(--chakra-spacing-2)',
      flexShrink: 0,
      justifyContent: 'flex-start',
      cursor: 'switch',
      borderRadius: 'full',
      position: 'relative',
      width: 'var(--switch-width)',
      height: 'var(--switch-height)',
      _disabled: {
        opacity: 'control.disabled',
        cursor: 'not-allowed',
      },
      _invalid: {
        outline: 'var(--chakra-border-widths-2) solid',
        outlineColor: 'border.error',
        outlineOffset: 'var(--chakra-spacing-0\\.5)',
      },
    },

    thumb: {
      _motionReduce: { transitionProperty: 'color, background-color, border-color, opacity' },
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexShrink: 0,
      transitionProperty: 'translate',
      transitionDuration: 'fast',
      borderRadius: 'inherit',
      _checked: {
        translate: 'var(--switch-x) 0',
      },
    },
  },

  variants: {
    variant: {
      primary: {
        control: {
          borderRadius: 'full',
          bg: 'switch.primary.bg',
          focusVisibleRing: 'outside',
          _checked: {
            bg: 'selected.option.bg',
            _hover: {
              bg: 'hover',
            },
          },
        },
        thumb: {
          bg: 'white',
          width: 'var(--switch-height)',
          height: 'var(--switch-height)',
          scale: '0.8',
          boxShadow: 'sm',
          _checked: {
            bg: 'white',
          },
        },
      },
    },

    direction: {
      rtl: {
        root: {
          flexDirection: 'row-reverse',
        },
      },
      ltr: {
        root: {
          flexDirection: 'row',
        },
      },
    },

    size: {
      sm: {
        root: {
          '--switch-width': 'var(--chakra-spacing-6\\.5)',
          '--switch-height': 'sizes.4',
          '--switch-indicator-font-size': 'fontSizes.sm',
        },
        label: {
          textStyle: 'sm',
        },
      },
      md: {
        root: {
          '--switch-width': 'var(--chakra-spacing-8\\.5)',
          '--switch-height': 'sizes.5',
          '--switch-indicator-font-size': 'fontSizes.md',
        },
        label: {
          textStyle: 'sm',
        },
      },
      lg: {
        root: {
          '--switch-width': 'var(--chakra-spacing-12\\.5)',
          '--switch-height': 'sizes.7',
          '--switch-indicator-font-size': 'fontSizes.md',
        },
        label: {
          textStyle: 'md',
        },
      },
    },
  },

  defaultVariants: {
    variant: 'primary',
    direction: 'ltr',
    size: 'md',
  },
});
