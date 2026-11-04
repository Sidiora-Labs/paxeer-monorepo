import { unparse } from 'papaparse';
import React from 'react';

import { Link } from 'toolkit/chakra/link';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  address: string;
  type: 'internal-transactions' | 'token-transfers';
  items: ReadonlyArray<object> | undefined;
  isLoading: boolean;
}

export function addressPageCsv(items: ReadonlyArray<object>): string {
  const fields = Array.from(new Set(items.flatMap(item => Object.keys(item))));
  const data = items.map(item => {
    const values = Object.fromEntries(Object.entries(item));
    return fields.map(field => {
      const value: unknown = values[field];
      return value !== null && typeof value === 'object' ? JSON.stringify(value) : value ?? '';
    });
  });
  return unparse({ fields, data }, { escapeFormulae: true });
}

const AddressPageCsvExport = ({ address, type, items, isLoading }: Props) => {
  const href = React.useMemo(() => items && !isLoading && items.length ?
    `data:text/csv;charset=utf-8,${ encodeURIComponent(addressPageCsv(items)) }` : undefined, [ items, isLoading ]);

  return (
    <Link
      href={ href }
      download={ `${ address }_${ type }_page.csv` }
      aria-disabled={ !href }
      loading={ isLoading }
      whiteSpace="nowrap"
      textStyle="sm"
    >
      <IconSvg name="files/csv" boxSize={ 5 }/>
      Download Page Data
    </Link>
  );
};

export default React.memo(AddressPageCsvExport);
