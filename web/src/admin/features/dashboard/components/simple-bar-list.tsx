import { cn } from '@/admin/lib/utils'

/** shadcn-admin's analytics bar list. Bar widths are set through the CSSOM, which the CSP allows. */
export function SimpleBarList({
  items,
  valueFormatter,
  barClass,
}: {
  items: { name: string; value: number; href?: string }[]
  valueFormatter: (n: number) => string
  barClass: string
}) {
  const max = Math.max(...items.map((i) => i.value), 1)
  return (
    <ul className='space-y-3'>
      {items.map((i) => (
        <li key={i.name} className='flex items-center justify-between gap-3'>
          <div className='min-w-0 flex-1'>
            <div className='mb-1 truncate text-xs text-muted-foreground'>{i.name}</div>
            <div className='h-2.5 w-full rounded-full bg-muted'>
              <div
                className={cn('h-2.5 rounded-full', barClass)}
                style={{ width: `${Math.round((i.value / max) * 100)}%` }}
              />
            </div>
          </div>
          <div className='ps-2 text-xs font-medium tabular-nums'>{valueFormatter(i.value)}</div>
        </li>
      ))}
    </ul>
  )
}
