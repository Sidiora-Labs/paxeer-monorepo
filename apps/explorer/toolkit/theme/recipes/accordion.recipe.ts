import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  className: 'chakra-accordion',
  slots: [ 'root', 'item', 'itemTrigger', 'itemContent', 'itemBody', 'itemIndicator' ],
  base: {
    root: {
      width: 'full',
      '--accordion-radius': 'none',
    },
    item: {
      overflowAnchor: 'none',
      borderColor: 'border.divider',
    },
    itemTrigger: {
      display: 'flex',
      alignItems: 'center',
      width: 'full',
      outline: '0',
      gap: '1',
      fontWeight: 'medium',
      borderRadius: 'var(--accordion-radius)',
      cursor: 'pointer',
      _focusVisible: {
        outline: 'var(--chakra-border-widths-2) solid',
        outlineColor: 'focusRing',
      },
    },
    itemBody: {
      pt: '0',
      pb: 'var(--accordion-padding-y)',
    },
    itemContent: {
      overflow: 'hidden',
      borderRadius: 'var(--accordion-radius)',
      _open: {
        _motionSafe: { animationName: 'expand-height, fade-in' },
        animationDuration: 'moderate',
      },
      _closed: {
        _motionSafe: { animationName: 'collapse-height, fade-out' },
        animationDuration: 'moderate',
      },
    },
    itemIndicator: {
      _motionReduce: { transitionProperty: 'color, background-color, border-color, opacity' },
      transition: 'rotate var(--chakra-durations-normal) var(--chakra-easings-standard)',
      transformOrigin: 'center',
    },
  },

  variants: {
    noAnimation: {
      'true': {
        itemContent: {
          _open: {
            animationName: 'none',
          },
          _closed: {
            animationName: 'none',
          },
        },
        itemIndicator: {
          transition: 'none',
        },
      },
    },
    variant: {
      outline: {
        item: {
          borderBottomWidth: 'var(--chakra-border-widths-1)',
        },
        itemIndicator: {
          color: 'gray.500',
        },
      },
      faq: {
        item: {
          borderBottomWidth: 'var(--chakra-border-widths-1)',
        },
        itemTrigger: {
          textStyle: 'heading.md',
        },
        itemIndicator: {
          color: 'link.primary',
          _groupHover: {
            color: 'link.primary.hover',
          },
        },
      },
    },

    size: {
      sm: {
        root: {
          '--accordion-padding-x': '0',
          '--accordion-padding-y': 'spacing.2',
        },
        itemTrigger: {
          textStyle: 'sm',
          py: 'var(--accordion-padding-y)',
        },
        itemIndicator: {
          boxSize: '5',
        },
      },
      md: {
        root: {
          '--accordion-padding-x': '0',
          '--accordion-padding-y': 'spacing.3',
        },
        itemTrigger: {
          textStyle: 'md',
          py: 'var(--accordion-padding-y)',
        },
        itemIndicator: {
          boxSize: '5',
        },
      },
    },
  },

  compoundVariants: [
    {
      variant: 'faq',
      size: 'md',
      css: {
        itemIndicator: {
          boxSize: 'var(--chakra-spacing-3\\.5)',
          margin: 'var(--chakra-spacing-1\\.25)',
        },
        itemBody: {
          paddingLeft: 'var(--chakra-spacing-9)',
        },
        itemTrigger: {
          gap: '3',
        },
      },
    },
  ],

  defaultVariants: {
    size: 'md',
    variant: 'outline',
  },
});
