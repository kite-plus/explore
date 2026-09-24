import { useEffect, useState } from 'react'
import { formatDateTime, formatRelative } from '@/admin/lib/format'

/** Relative time that refreshes every minute; the title holds the exact time. */
export function TimeAgo({ iso, className }: { iso: string | null | undefined; className?: string }) {
  const [now, setNow] = useState(Date.now)

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 60_000)
    return () => window.clearInterval(timer)
  }, [])

  if (!iso) return <span className={className}>—</span>
  return (
    <time dateTime={iso} title={formatDateTime(iso)} className={className}>
      {formatRelative(iso, now)}
    </time>
  )
}
