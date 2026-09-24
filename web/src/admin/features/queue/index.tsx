import { Activity, CirclePause, Clock, Server, TriangleAlert } from 'lucide-react'
import { useAdminQuery } from '@/admin/lib/api'
import { Link } from '@/admin/router'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { QueryError } from '@/admin/components/query-error'
import { TimeAgo } from '@/admin/components/time-ago'
import { StatCard } from '@/admin/features/dashboard/components/stat-card'
import type { FetchQueueSnapshot } from '@/lib/admin-types'
import { AttemptsDrawer } from './components/attempts-drawer'
import { QueueProvider, useQueue } from './components/queue-provider'
import { QueueTable } from './components/queue-table'

export function Queue() {
  return (
    <QueueProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='抓取任务' description='调度、进程心跳和最近的执行结果，每 15 秒刷新一次。' />
        <QueueContent />
      </Main>
    </QueueProvider>
  )
}

function QueueContent() {
  const { attemptsFor, setAttemptsFor } = useQueue()
  const { data, isPending, error, refetch } = useAdminQuery<FetchQueueSnapshot>('/fetch-queue', {
    refetchInterval: 15_000,
  })
  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />
  const items = data?.data ?? []
  const count = (status: string) => items.filter((item) => item.queue_status === status).length

  return (
    <>
      {data?.crawler_paused && (
        <Alert>
          <CirclePause />
          <AlertTitle>抓取任务已暂停</AlertTitle>
          <AlertDescription>
            <p>
              抓取进程不会领取新任务，队列保留。<Link to='/admin/settings'>前往系统设置恢复</Link>
            </p>
          </AlertDescription>
        </Alert>
      )}
      {data && !data.worker_online && (
        <Alert variant='destructive'>
          <TriangleAlert />
          <AlertTitle>抓取进程离线</AlertTitle>
          <AlertDescription>待执行和等待重试的任务暂不会运行。请检查 worker 进程和日志。</AlertDescription>
        </Alert>
      )}
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <StatCard
          title='抓取进程'
          value={data ? (data.worker_online ? `${data.worker_count} 个在线` : '离线') : undefined}
          hint={
            data?.worker_last_seen_at ? (
              <>
                最近心跳 <TimeAgo iso={data.worker_last_seen_at} />
              </>
            ) : (
              '还没有心跳记录'
            )
          }
          icon={Server}
        />
        <StatCard title='待执行' value={data ? count('queued') + count('stalled') : undefined} hint='已到调度时间' icon={Clock} />
        <StatCard title='抓取中' value={data ? count('running') : undefined} hint='已被进程领取' icon={Activity} />
        <StatCard
          title='连续失败'
          value={data ? items.filter((item) => item.consecutive_failures > 0).length : undefined}
          hint={data ? `其中 ${count('retry')} 个等待重试` : ''}
          icon={TriangleAlert}
        />
      </div>
      <QueueTable data={items} loading={isPending} />
      {attemptsFor && (
        <AttemptsDrawer
          key={attemptsFor.host}
          open
          onOpenChange={(open) => !open && setAttemptsFor(null)}
          item={attemptsFor}
        />
      )}
    </>
  )
}
