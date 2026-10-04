import '@tanstack/react-table'

declare module '@tanstack/react-table' {
  interface ColumnMeta<TData, TValue> {
    title?: string // the column's name in the column toggle
    className?: string // apply to both th and td
    tdClassName?: string
    thClassName?: string
  }

  interface TableMeta<TData> {
    // Faceted filter counts from the API, by column id and then option value.
    facetCounts?: Record<string, Record<string, number>>
  }
}
