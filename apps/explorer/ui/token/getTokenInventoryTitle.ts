import { formatScanTableCount } from 'ui/shared/scan/ScanTableCard';

type Params = {
  itemsNum: number;
  hasNextPage: boolean;
  inventoryCount?: number;
};

export default function getTokenInventoryTitle({ itemsNum, hasNextPage, inventoryCount }: Params): string {
  if (inventoryCount !== undefined) {
    return formatScanTableCount(itemsNum < inventoryCount ?
      { kind: 'latest', value: inventoryCount, itemsName: 'tokens', shownValue: itemsNum } :
      { kind: 'total', value: inventoryCount, itemsName: 'tokens' });
  }

  if (hasNextPage) {
    return formatScanTableCount({ kind: 'more_than', value: itemsNum, itemsName: 'tokens' });
  }

  return `Showing ${ itemsNum.toLocaleString() } tokens on this page`;
}
