import { defineSlotRecipe } from '@chakra-ui/react';

export const recipe = defineSlotRecipe({
  slots: [ 'root', 'row', 'cell', 'columnHeader', 'caption', 'footer', 'body', 'header' ],
  base: {
    root: {
      tableLayout: 'fixed',
      fontVariant: 'normal',
      textStyle: 'small',
      color: 'text.primary',
      fontVariantLigatures: 'no-contextual',
      borderCollapse: 'collapse',
      width: 'full',
      textAlign: 'start',
      verticalAlign: 'top',
      overflow: 'unset',
    },
    cell: {
      textAlign: 'start',
      alignItems: 'center',
      verticalAlign: 'top',
      fontWeight: 'medium',
    },
    columnHeader: {
      fontWeight: 'medium',
      textAlign: 'start',
    },
  },

  variants: {
    variant: {
      line: {
        columnHeader: {
          color: 'table.header.fg',
          backgroundColor: 'table.header.bg',
          _first: {
            borderTopLeftRadius: 'sm',
          },
          _last: {
            borderTopRightRadius: 'sm',
          },
        },
        cell: {
          borderBottomWidth: 'var(--chakra-border-widths-1)',
          borderColor: 'border.divider',
        },
        row: {
          bg: 'bg.surface',
        },
      },
      scan: {
        root: {
          bg: 'bg.surface',
        },
        columnHeader: {
          color: 'table.header.fg',
          backgroundColor: 'table.header.bg',
          textStyle: 'xs',
          fontWeight: 'medium',
          letterSpacing: 'wide',
          borderBottomWidth: 'var(--chakra-border-widths-1)',
          borderColor: 'border.strong',
          _first: {
            borderTopLeftRadius: 'sm',
          },
          _last: {
            borderTopRightRadius: 'sm',
          },
        },
        cell: {
          color: 'text.primary',
          borderBottomWidth: 'var(--chakra-border-widths-1)',
          borderColor: 'border.divider',
        },
        row: {
          bg: 'bg.surface',
          _hover: {
            bg: 'table.row.hover',
          },
          _last: {
            '& td': {
              borderBottomWidth: '0',
            },
          },
        },
      },
    },

    size: {
      md: {
        root: {
          fontSize: 'sm',
        },
        columnHeader: {
          px: 'var(--chakra-spacing-1\\.5)',
          py: 'var(--chakra-spacing-2\\.5)',
          _first: {
            pl: 3,
          },
          _last: {
            pr: 3,
          },
        },
        cell: {
          px: 'var(--chakra-spacing-1\\.5)',
          py: 4,
          _first: {
            pl: 3,
          },
          _last: {
            pr: 3,
          },
        },
      },
    },
  },

  // Apply the visual system after the compatibility variants and size declarations.
  compoundVariants: [
    {
      variant: [ 'line', 'scan' ],
      css: {
        root: { bg: 'canvas', borderTopWidth: 'var(--chakra-border-widths-1)', borderColor: 'hairline' },
        header: { height: 'var(--chakra-spacing-9)' },
        columnHeader: {
          textStyle: 'label',
          height: 'var(--chakra-spacing-9)',
          py: 0,
          color: 'text.muted',
          backgroundColor: 'canvas',
          borderBottomWidth: 'var(--chakra-border-widths-1)',
          borderColor: 'hairline',
          _first: { borderTopLeftRadius: 'none' },
          _last: { borderTopRightRadius: 'none' },
          '&[data-is-numeric=true], &[data-numeric]': { textAlign: 'right' },
        },
        row: { height: 'var(--chakra-spacing-11)', bg: 'canvas', _hover: { bg: 'raised' } },
        cell: {
          textStyle: 'small',
          height: 'var(--chakra-spacing-11)',
          py: 'var(--chakra-spacing-3)',
          verticalAlign: 'middle',
          borderBottomWidth: 'var(--chakra-border-widths-1)',
          borderColor: 'hairline',
          '&[data-is-numeric=true], &[data-numeric]': { textAlign: 'right', textStyle: 'number' },
        },
      },
    },
  ],

  defaultVariants: {
    variant: 'line',
    size: 'md',
  },
});
