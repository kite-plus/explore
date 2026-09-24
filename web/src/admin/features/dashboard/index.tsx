import { Flag, Inbox, Rss, Server } from 'lucide-react'
import { useAdminQuery } from '@/admin/lib/api'
import { Link } from '@/admin/router'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/admin/components/ui/card'
import { Skeleton } from '@/admin/components/ui/skeleton'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { RefreshButton } from '@/admin/components/refresh-button'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminBlog, AdminOverview, FetchQueueSnapshot } from '@/lib/admin-types'
import { FailingBlogs } from './components/failing-blogs'
import { QueueStates } from './components/queue-states'
import { RecentBlogs } from './components/recent-blogs'
import { StatCard, type Tone } from './components/stat-card'
import { VisibleBlogs } from './components/visible-blogs'

const count = (value: number) => value.toLocaleString('zh-CN')

export function Dashboard() {
  const { data: overview } = useAdminQuery<AdminOverview>('/overview')
  const { data: blogs } = useAdminQuery<{ data: AdminBlog[] }>('/blogs')
  const { data: queue } = useAdminQuery<FetchQueueSnapshot>('/fetch-queue')
  const stats = overview?.stats
  const recent = [...(blogs?.data ?? [])]
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
    .slice(0, 6)

  return (
    <>
      <AppHeader fixed={false} />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle
          title='工作台'
          description={
            stats
              ? `${count(stats.blogs)} 个博客 · ${count(stats.entries)} 篇文章 · ${count(stats.users)} 位注册用户`
              : '正在读取统计…'
          }
        >
          <RefreshButton />
        </PageTitle>

        <div className='grid grid-cols-2 gap-4 lg:grid-cols-4'>
          <StatCard
            title='待审核收录'
            value={stats?.pending_submissions}
            hint={stats?.pending_submissions ? '读者提交，等待审核' : '没有等待审核的投稿'}
            icon={Inbox}
            tone={stats?.pending_submissions ? 'warning' : 'success'}
            to='/admin/submissions'
          />
          <StatCard
            title='待处理下架'
            value={stats?.pending_takedowns}
            hint={stats?.pending_takedowns ? '读者举报和内部申请' : '没有待处理的申请'}
            icon={Flag}
            tone={stats?.pending_takedowns ? 'warning' : 'success'}
            to='/admin/takedowns'
          />
          <StatCard
            title='抓取异常'
            value={stats?.failing_blogs}
            hint={stats?.failing_blogs ? '上次抓取失败的博客' : '所有博客抓取正常'}
            icon={Rss}
            tone={stats?.failing_blogs ? 'error' : 'success'}
            to='/admin/queue'
          />
          <WorkerCard overview={overview} />
        </div>

        <div className='grid gap-4 lg:grid-cols-7'>
          <Card className='lg:col-span-4'>
            <CardHeader>
              <CardTitle>最近收录</CardTitle>
              <CardDescription>最新加入目录的博客。</CardDescription>
              <CardAction>
                <Link to='/admin/blogs' className='text-sm text-muted-foreground hover:text-foreground'>
                  查看全部
                </Link>
              </CardAction>
            </CardHeader>
            <CardContent>{blogs ? <RecentBlogs blogs={recent} /> : <ListSkeleton rows={6} />}</CardContent>
          </Card>
          <div className='flex flex-col gap-4 lg:col-span-3'>
            {blogs ? (
              <VisibleBlogs blogs={blogs.data} />
            ) : (
              <Card className='px-6'>
                <ListSkeleton rows={2} />
              </Card>
            )}
            <Card className='flex-1'>
              <CardHeader>
                <CardTitle>抓取队列</CardTitle>
                <CardDescription>
                  {queue ? `共 ${count(queue.data.length)} 个抓取任务。` : '正在读取…'}
                </CardDescription>
              </CardHeader>
              <CardContent>{queue ? <QueueStates items={queue.data} /> : <ListSkeleton rows={3} />}</CardContent>
            </Card>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>抓取失败的博客</CardTitle>
            <CardDescription>按连续失败次数排序，抓取成功后清零。</CardDescription>
            <CardAction>
              <Link to='/admin/queue' className='text-sm text-muted-foreground hover:text-foreground'>
                查看队列
              </Link>
            </CardAction>
          </CardHeader>
          <CardContent>{queue ? <FailingBlogs items={queue.data} /> : <ListSkeleton rows={3} />}</CardContent>
        </Card>
      </Main>
    </>
  )
}

/** The crawler's state; being offline outranks being paused. */
function WorkerCard({ overview }: { overview: AdminOverview | undefined }) {
  if (!overview) {
    return <StatCard title='抓取进程' value={undefined} hint='正在读取…' icon={Server} tone='neutral' to='/admin/queue' />
  }
  const heartbeat = overview.worker_last_seen_at ? (
    <>
      最近心跳 <TimeAgo iso={overview.worker_last_seen_at} />
    </>
  ) : (
    '还没有心跳记录'
  )
  const [value, tone, hint, to]: [string, Tone, React.ReactNode, string] = !overview.worker_online
    ? ['离线', 'error', heartbeat, '/admin/queue']
    : overview.crawler_paused
      ? ['已暂停', 'warning', '暂停领取，队列保留', '/admin/settings']
      : [`${overview.worker_count} 个在线`, 'success', heartbeat, '/admin/queue']
  return <StatCard title='抓取进程' value={value} hint={hint} icon={Server} tone={tone} to={to} />
}

function ListSkeleton({ rows }: { rows: number }) {
  return (
    <div className='flex flex-col gap-2'>
      {Array.from({ length: rows }, (_, i) => (
        <Skeleton key={i} className='h-9 w-full' />
      ))}
    </div>
  )
}
