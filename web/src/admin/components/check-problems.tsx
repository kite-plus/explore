import { CircleAlert, Info, TriangleAlert } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import type { Problem } from '@/lib/types'

const SEVERITY = {
  error: { icon: CircleAlert, className: 'text-destructive', label: '错误' },
  warning: { icon: TriangleAlert, className: 'text-warning', label: '警告' },
  info: { icon: Info, className: 'text-muted-foreground', label: '提示' },
} as const

const ORDER = { error: 0, warning: 1, info: 2 } as const

export function sortProblems(problems: Problem[] | null | undefined) {
  return [...(problems ?? [])].sort((a, b) => ORDER[a.severity] - ORDER[b.severity])
}

export function problemCounts(problems: Problem[] | null | undefined) {
  const list = problems ?? []
  return {
    errors: list.filter((p) => p.severity === 'error').length,
    warnings: list.filter((p) => p.severity === 'warning').length,
  }
}

/** A check report's problems, errors first. */
export function CheckProblems({ problems }: { problems: Problem[] | null | undefined }) {
  const sorted = sortProblems(problems)
  if (sorted.length === 0) return <p className='text-sm text-muted-foreground'>没有发现问题。</p>
  return (
    <ul className='space-y-2'>
      {sorted.map((problem, index) => {
        const { icon: Icon, className, label } = SEVERITY[problem.severity] ?? SEVERITY.info
        return (
          <li key={`${problem.code}-${index}`} className='flex items-start gap-2 text-sm'>
            <Icon className={cn('mt-0.5 size-4 shrink-0', className)} aria-label={label} />
            <span className='min-w-0'>
              {problem.detail || problem.hint || problem.code}
              {problem.count && problem.count > 1 ? (
                <span className='ms-1 text-muted-foreground tabular-nums'>×{problem.count}</span>
              ) : null}
            </span>
          </li>
        )
      })}
    </ul>
  )
}
