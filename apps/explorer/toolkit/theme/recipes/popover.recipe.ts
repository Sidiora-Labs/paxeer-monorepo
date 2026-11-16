import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  slots: [ 'content', 'header', 'body', 'footer', 'arrow', 'arrowTip' ],
  base: {
    content: {
      position: 'relative',
      display: 'flex',
      flexDirection: 'column',
      textStyle: 'sm',
      '--popover-bg': 'colors.popover.bg',
      bg: 'var(--popover-bg)',
      boxShadow: 'popover',
      boxShadowColor: 'colors.popover.shadow',
      '--popover-mobile-size': 'calc(100dvw - var(--chakra-spacing-4))',
      width: 'fit-content',
      maxW: {
        base: 'calc(100vw - var(--chakra-spacing-2))',
        lg: 'var(--chakra-spacing-200)',
      },
      borderWidth: '0',
      borderRadius: 'md',
      '--popover-z-index': 'zIndex.popover',
      zIndex: 'calc(var(--popover-z-index) + var(--layer-index, 0))',
      outline: '0',
      transformOrigin: 'var(--transform-origin)',
      _open: {
        _motionSafe: { animationStyle: 'scale-fade-in' },
        animationDuration: 'fast',
      },
      _closed: {
        _motionSafe: { animationStyle: 'scale-fade-out' },
        animationDuration: 'faster',
      },
    },
    header: {
      paddingInline: 'var(--popover-padding)',
      paddingTop: 'var(--popover-padding)',
    },
    body: {
      padding: 'var(--popover-padding)',
      flex: '1',
    },
    footer: {
      display: 'flex',
      alignItems: 'center',
      paddingInline: 'var(--popover-padding)',
      paddingBottom: 'var(--popover-padding)',
    },
    arrow: {
      '--arrow-size': 'sizes.3',
      '--arrow-background': 'var(--popover-bg)',
    },
    arrowTip: {
      borderTopWidth: 'var(--chakra-border-widths-1)',
      borderInlineStartWidth: 'var(--chakra-border-widths-1)',
    },
  },
  variants: {
    size: {
      sm: {
        content: {
          '--popover-padding': 'spacing.4',
        },
      },
    },
  },
  defaultVariants: {
    size: 'sm',
  },
});
