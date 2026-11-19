import { defineRecipe } from '@chakra-ui/react';

export const recipe = defineRecipe({
  base: {
    display: 'flex',
    gap: 0,
    minH: 'touchTarget',
    minW: 'touchTarget',
    fontWeight: 'medium',
    focusRingColor: 'focusRing',
    overflow: 'hidden',
    borderRadius: 'full',
    _focusVisible: { outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-spacing-0\\.5)' },
    _disabled: {
      opacity: 'control.disabled',
    },
  },
  variants: {
    variant: {
      solid: {
        bg: 'button.solid.bg',
        color: 'button.solid.text',
        _hover: {
          bg: 'button.solid.bg.hover',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'button.solid.text',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
        _expanded: {
          bg: 'button.solid.bg.hover',
        },
      },
      solid_danger: {
        bg: 'negative',
        color: 'button.solid.text',
        _hover: {
          bg: 'negative',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'button.solid.text',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
        _expanded: {
          bg: 'negative',
        },
      },
      outline: {
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        bg: 'transparent',
        color: 'button.outline.fg',
        borderColor: 'border.strong',
        _hover: {
          bg: 'transparent',
          color: 'hover',
          borderColor: 'hover',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'button.outline.fg',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
      },
      outline_danger: {
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        bg: 'transparent',
        color: 'negative',
        borderColor: 'negative',
        _hover: {
          bg: 'transparent',
          color: 'negative',
          borderColor: 'negative',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'negative',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
      },
      dropdown: {
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        bg: 'transparent',
        color: 'button.dropdown.fg',
        borderColor: 'button.dropdown.border',
        _hover: {
          bg: 'transparent',
          color: 'hover',
          borderColor: 'hover',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'selected.option.bg',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
        // When the dropdown is open, the button should be active
        _expanded: {
          bg: 'transparent',
          color: 'hover',
          borderColor: 'hover',
        },
        // We have a special state for this button variant that serves as a popover trigger.
        // When any items (filters) are selected in the popover, the button should change its background and text color.
        // The last CSS selector is for redefining styles for the TabList component.
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          borderColor: 'transparent',
          _hover: {
            bg: 'selected.control.bg',
            color: 'hover',
            borderColor: 'transparent',
          },
          _expanded: {
            color: 'hover',
          },
        },
      },
      header: {
        bg: 'transparent',
        color: 'button.header.fg',
        borderColor: 'button.header.border',
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        _hover: {
          bg: 'transparent',
          color: 'hover',
          borderColor: 'hover',
        },
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'selected.option.bg',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
        _selected: {
          bg: 'button.header.bg.selected',
          color: 'button.header.fg.selected',
          borderColor: 'transparent',
          borderWidth: 'var(--chakra-border-widths-0)',
          _hover: {
            bg: 'button.header.bg.selected',
            color: 'hover',
          },
          _expanded: {
            color: 'hover',
          },
          _highlighted: {
            bg: 'button.header.bg.highlighted',
            color: 'button.header.fg.highlighted',
            borderColor: 'transparent',
            borderWidth: 'var(--chakra-border-widths-0)',
            _expanded: {
              color: 'hover',
            },
            _hover: {
              bg: 'button.header.bg.highlighted',
              color: 'hover',
            },
          },
        },
      },
      hero: {
        bg: 'button.hero.bg',
        color: 'button.hero.fg',
        _loading: {
          opacity: 1,
          '& .chakra-spinner': {
            borderColor: 'button.hero.fg',
            borderBottomColor: 'spinner.track',
            borderInlineStartColor: 'spinner.track',
          },
        },
        _hover: {
          bg: 'button.hero.bg.hover',
          color: 'button.hero.fg.hover',
        },
        _selected: {
          bg: 'button.hero.bg.selected',
          color: 'button.hero.fg.selected',
          _hover: {
            bg: 'button.hero.bg.selected',
            color: 'hover',
          },
          _expanded: {
            color: 'hover',
          },
        },
      },
      segmented: {
        bg: 'transparent',
        color: 'button.segmented.fg',
        borderColor: 'selected.control.bg',
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        borderRadius: 'none',
        _hover: {
          color: 'hover',
        },
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          _hover: {
            bg: 'selected.control.bg',
            color: 'selected.control.text',
          },
          '& + *': {
            borderLeftWidth: '0',
          },
        },
        _notLast: {
          borderRightWidth: '0',
          _selected: {
            borderRightWidth: 'var(--chakra-border-widths-1)',
          },
        },
        _first: {
          borderTopLeftRadius: 'full',
          borderBottomLeftRadius: 'full',
        },
        _last: {
          borderTopRightRadius: 'full',
          borderBottomRightRadius: 'full',
        },
      },
      scan_control: {
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        bg: 'bg.surface',
        color: 'text.secondary',
        borderColor: 'border.divider',
        fontWeight: 'medium',
        _hover: {
          bg: 'bg.surface',
          color: 'hover',
          borderColor: 'border.strong',
        },
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          borderColor: 'transparent',
          _hover: {
            bg: 'selected.control.bg',
            color: 'selected.control.text',
            borderColor: 'transparent',
          },
        },
        _disabled: {
          bg: 'bg.surface',
          color: 'text.muted',
          borderColor: 'border.divider',
        },
        _loading: {
          opacity: 1,
        },
      },
      plain: {
        bg: 'transparent',
        color: 'inherit',
        border: 'none',
        _hover: {
          bg: 'transparent',
        },
      },
      subtle: {
        bg: 'button.subtle.bg',
        color: 'button.subtle.fg',
        _hover: {
          bg: 'button.subtle.bg',
          color: 'hover',
        },
        _disabled: {
          bg: 'button.subtle.bg',
          color: 'button.subtle.fg',
        },
      },
      link: {
        bg: 'transparent',
        color: 'link.primary',
        border: 'none',
        fontWeight: 'normal',
        px: 0,
        h: 'auto',
        _hover: {
          bg: 'transparent',
          color: 'link.primary.hover',
        },
        _disabled: {
          color: 'text.secondary',
        },
      },
      icon_secondary: {
        bg: 'transparent',
        color: 'icon.secondary',
        border: 'none',
        _hover: {
          color: 'hover',
        },
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          _hover: {
            bg: 'selected.control.bg',
            color: 'hover',
          },
          _expanded: {
            color: 'hover',
          },
        },
        _expanded: {
          color: 'hover',
        },
      },
      icon_background: {
        bg: 'button.icon_background.bg',
        color: 'icon.secondary',
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        borderColor: 'border.strong',
        _hover: {
          color: 'hover',
        },
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          _hover: {
            bg: 'selected.control.bg',
            color: 'hover',
          },
          _expanded: {
            color: 'hover',
          },
        },
        _expanded: {
          color: 'hover',
        },
      },
      pagination: {
        borderWidth: 'var(--chakra-border-widths-1)',
        borderStyle: 'solid',
        bg: 'transparent',
        color: 'button.pagination.fg',
        borderColor: 'button.pagination.border',
        _hover: {
          bg: 'transparent',
          color: 'hover',
          borderColor: 'hover',
        },
        _selected: {
          bg: 'selected.control.bg',
          color: 'selected.control.text',
          borderColor: 'transparent',
          _hover: {
            bg: 'selected.control.bg',
            color: 'selected.control.text',
            borderColor: 'transparent',
          },
        },
      },
    },
    size: {
      '2xs_alt': {},

      '2xs': {
        px: 2,
        h: 5,
        minW: 5,
        textStyle: 'xs',
        borderRadius: 'full',
        gap: 1,
        _icon: { boxSize: 'auto' },
      },
      xs: {
        px: 2,
        h: 6,
        minW: 6,
        textStyle: 'sm',
        borderRadius: 'full',
        gap: 1,
        _icon: { boxSize: 'auto' },
      },
      sm: {
        px: 4,
        h: 8,
        minW: 8,
        textStyle: 'sm',
        borderRadius: 'full',
        gap: 1,
        _icon: { boxSize: 'auto' },
      },
      md: {
        px: 6,
        h: 10,
        minW: 10,
        textStyle: 'md',
        borderRadius: 'full',
        gap: 2,
        _icon: { boxSize: 'auto' },
        '& .chakra-spinner': { '--spinner-size': 'var(--chakra-spacing-5)' },
      },
    },
  },
  compoundVariants: [
    {
      size: [ '2xs_alt', '2xs', 'xs', 'sm', 'md' ],
      css: {
        borderRadius: 'control',
        _first: { borderTopLeftRadius: 'control', borderBottomLeftRadius: 'control' },
        _last: { borderTopRightRadius: 'control', borderBottomRightRadius: 'control' },
      },
    },
    { variant: [ 'outline', 'dropdown', 'header', 'pagination' ], css: { borderColor: 'border.strong' } },
  ],
  defaultVariants: {
    size: 'md',
    variant: 'solid',
  },
});
