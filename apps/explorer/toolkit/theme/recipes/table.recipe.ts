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
          borderBottomWidth: '1px',
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
          fontWeight: '500',
          letterSpacing: 'wide',
          borderBottomWidth: '1px',
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
          borderBottomWidth: '1px',
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
          px: '6px',
          py: '10px',
          _first: {
            pl: 3,
          },
          _last: {
            pr: 3,
          },
        },
        cell: {
          px: '6px',
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
        root: { bg: 'canvas', borderTopWidth: '1px', borderColor: 'hairline' },
        header: { height: '36px' },
        columnHeader: {
          textStyle: 'label',
          height: '36px',
          py: 0,
          color: 'text.muted',
          backgroundColor: 'canvas',
          borderBottomWidth: '1px',
          borderColor: 'hairline',
          _first: { borderTopLeftRadius: 'none' },
          _last: { borderTopRightRadius: 'none' },
          '&[data-is-numeric=true], &[data-numeric]': { textAlign: 'right' },
        },
        row: { height: '44px', bg: 'canvas', _hover: { bg: 'raised' } },
        cell: {
          textStyle: 'small',
          height: '44px',
          py: '12px',
          verticalAlign: 'middle',
          borderBottomWidth: '1px',
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
