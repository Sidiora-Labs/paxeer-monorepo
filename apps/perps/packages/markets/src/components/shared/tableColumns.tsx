import type { Column } from "@paxeer-network/ui";

export function withMarketTableControls<RecordType>(
  columns: Column<RecordType>[],
): Column<RecordType>[] {
  return columns.map((column) => {
    if (
      !column.onSort ||
      column.multiSort ||
      typeof column.title !== "string"
    ) {
      return column;
    }
    return {
      ...column,
      plantTextTitle: column.plantTextTitle ?? column.title,
      title: (
        <button type="button" className="oui-text-start">
          {column.title}
        </button>
      ),
    };
  });
}
