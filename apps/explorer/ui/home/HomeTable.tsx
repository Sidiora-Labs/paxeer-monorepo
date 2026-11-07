import React from 'react';

import { TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';

interface Props {
  label: string;
  columns: Array<{ label: string; numeric?: boolean; width?: string }>;
  children: React.ReactNode;
}

const HomeTable = ({ label, columns, children }: Props) => (
  <TableRoot
    aria-label={ label }
    variant="line"
    css={{
      '& th': { h: '36px', textStyle: 'label', py: 0 },
      '& td': { h: '44px', py: '4px', verticalAlign: 'middle' },
      '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas', zIndex: 1 },
      '& tr:hover td': { bg: 'raised' },
    }}
  >
    <TableHeader>
      <TableRow>
        { columns.map((column) => (
          <TableColumnHeader key={ column.label } isNumeric={ column.numeric } width={ column.width }>{ column.label }</TableColumnHeader>
        )) }
      </TableRow>
    </TableHeader>
    { children }
  </TableRoot>
);

export default HomeTable;
