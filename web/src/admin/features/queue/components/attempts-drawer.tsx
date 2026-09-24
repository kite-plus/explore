import { Loader2 } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import { useAdminQuery } from '@/admin/lib/api'
import { formatDateTime } from '@/admin/lib/format'
import { Badge } from '@/admin/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/admin/components/ui/sheet'
import { TimeAgo } from '@/admin/components/time-ago'
import type { FetchAttempt, FetchQueueItem } from '@/lib/admin-types'
import { outcomes } from '../data/data'

type AttemptsDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: FetchQueueItem
}

export function AttemptsDrawer({ open, onOpenChange, item }: AttemptsDrawerProps) {
  const { data, isPending, error } = useAdminQuery<{ data: FetchAttempt[] }>(
    `/blogs/${encodeURIComponent(item.host)}/fetch-attempts`
  )

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col sm:max-w-lg'>
        <SheetHeader className='text-start'>
          <SheetTitle>执行记录</SheetTitle>
          <SheetDescription>
            {item.name || item.host}（<span className='font-mono'>{item.host}</span>）最近的抓取尝试。
          </SheetDescription>
        </SheetHeader>
        <div className='flex-1 overflow-y-auto px-4 pb-6'>
          {isPending ? (
            <p className='flex items-center gap-2 text-sm text-muted-foreground'>
              <Loader2 className='size-4 animate-spin' />
              正在读取…
            </p>
          ) : error ? (
            <p className='text-sm text-destructive'>{error.message}</p>
          ) : !data?.data.length ? (
            <p className='text-sm text-muted-foreground'>还没有执行记录，等待抓取进程领取。</p>
          ) : (
            <ol className='relative space-y-6 border-s ps-6'>
              {data.data.map((attempt) => {
                const outcome = outcomes[attempt.outcome]
                return (
                  <li key={attempt.id} className='relative'>
                    <span className={cn('absolute -start-[1.84rem] top-1.5 size-2.5 rounded-full border-2 border-background', outcome.dot)} />
                    <div className='flex flex-wrap items-center gap-2 text-sm'>
                      <Badge variant='outline' className={outcome.className}>{outcome.label}</Badge>
                      <TimeAgo iso={attempt.started_at} className='text-muted-foreground' />
                    </div>
                    <p className='mt-1 text-xs text-muted-foreground'>
                      {formatDateTime(attempt.started_at)}
                      {attempt.http_status !== null && ` · HTTP ${attempt.http_status}`}
                      {attempt.entry_count !== null && ` · ${attempt.entry_count} 篇文章`}
                    </p>
                    {attempt.error && (
                      <pre className='mt-2 rounded-md bg-muted p-2 font-mono text-xs break-all whitespace-pre-wrap text-destructive'>
                        {attempt.error}
                      </pre>
                    )}
                  </li>
                )
              })}
            </ol>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
