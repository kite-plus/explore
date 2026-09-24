import { Activity, Clock, CirclePause, Flag, Inbox, RefreshCw, TriangleAlert, Server, Loader2 } from 'lucide-react'
import { useIsFetching, useQueryClient } from '@tanstack/react-query'
import { useAdminQuery } from '@/admin/lib/api'
import { Link } from '@/admin/router'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { Button } from '@/admin/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/admin/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/admin/components/ui/tabs'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminBlog, AdminOverview, FetchQueueSnapshot } from '@/lib/admin-types'
import { GeneratorsChart } from './components/generators-chart'
import { RecentBlogs } from './components/recent-blogs'
import { SimpleBarList } from './components/simple-bar-list'
import { StatCard } from './components/stat-card'

export function Dashboard() {
  const queryClient = useQueryClient()
  const fetching = useIsFetching({ queryKey: ['admin'] }) > 0
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

      <Main>
        <div className='mb-2 flex items-center justify-between space-y-2'>
          <h1 className='text-2xl font-bold tracking-tight'>工作台</h1>
          <div className='flex items-center space-x-2'>
            <Button
              variant='outline'
              disabled={fetching}
              onClick={() => void queryClient.invalidateQueries({ queryKey: ['admin'] })}
            >
              {fetching ? <Loader2 className='animate-spin' /> : <RefreshCw />}
              刷新
            </Button>
          </div>
        </div>

        {overview?.crawler_paused && (
          <Alert className='mb-4'>
            <CirclePause />
            <AlertTitle>抓取任务已暂停</AlertTitle>
            <AlertDescription>
              <p>
                抓取进程不会领取新任务，队列保留。<Link to='/admin/settings'>前往系统设置恢复</Link>
              </p>
            </AlertDescription>
          </Alert>
        )}
        {overview && !overview.worker_online && (
          <Alert variant='destructive' className='mb-4'>
            <TriangleAlert />
            <AlertTitle>抓取进程离线</AlertTitle>
            <AlertDescription>任务会留在队列里，进程启动后继续执行。请检查 worker 进程和日志。</AlertDescription>
          </Alert>
        )}

        <Tabs orientation='vertical' defaultValue='overview' className='space-y-4'>
          <div className='w-full overflow-x-auto pb-2'>
            <TabsList>
              <TabsTrigger value='overview'>概览</TabsTrigger>
              <TabsTrigger value='crawler'>抓取状态</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value='overview' className='space-y-4'>
            <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
              <Link to='/admin/submissions' className='rounded-xl transition-opacity hover:opacity-80'>
                <StatCard
                  title='待审核收录'
                  value={stats?.pending_submissions}
                  hint='读者提交、等待审核的博客'
                  icon={Inbox}
                />
              </Link>
              <Link to='/admin/takedowns' className='rounded-xl transition-opacity hover:opacity-80'>
                <StatCard
                  title='待处理下架'
                  value={stats?.pending_takedowns}
                  hint='读者举报和内部申请'
                  icon={Flag}
                />
              </Link>
              <Link to='/admin/queue' className='rounded-xl transition-opacity hover:opacity-80'>
                <StatCard
                  title='抓取异常'
                  value={stats?.failing_blogs}
                  hint='最近一次抓取失败的博客'
                  icon={TriangleAlert}
                />
              </Link>
              <Link to='/admin/queue' className='rounded-xl transition-opacity hover:opacity-80'>
                <StatCard
                  title='到期待抓取'
                  value={stats?.due_fetches}
                  hint='已到调度时间、等待领取'
                  icon={Clock}
                />
              </Link>
            </div>
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
              <Card className='col-span-1 lg:col-span-4'>
                <CardHeader>
                  <CardTitle>博客程序分布</CardTitle>
                  <CardDescription>
                    已收录的 {stats?.blogs ?? '…'} 个博客所用的建站程序。
                  </CardDescription>
                </CardHeader>
                <CardContent className='ps-2'>
                  <GeneratorsChart blogs={blogs?.data ?? []} />
                </CardContent>
              </Card>
              <Card className='col-span-1 lg:col-span-3'>
                <CardHeader>
                  <CardTitle>最近收录</CardTitle>
                  <CardDescription>
                    缓存了 {stats?.entries.toLocaleString('zh-CN') ?? '…'} 篇文章，注册用户 {stats?.users ?? '…'} 人。
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <RecentBlogs blogs={recent} />
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value='crawler' className='space-y-4'>
            <CrawlerTab overview={overview} queue={queue} />
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}

function CrawlerTab({
  overview,
  queue,
}: {
  overview: AdminOverview | undefined
  queue: FetchQueueSnapshot | undefined
}) {
  const items = queue?.data ?? []
  const count = (status: string) => items.filter((item) => item.queue_status === status).length
  const failing = [...items]
    .filter((item) => item.consecutive_failures > 0)
    .sort((a, b) => b.consecutive_failures - a.consecutive_failures)
    .slice(0, 6)
  const states = [
    { name: '定时等待', value: count('scheduled') },
    { name: '待执行', value: count('queued') },
    { name: '抓取中', value: count('running') },
    { name: '等待重试', value: count('retry') },
    { name: '执行中断', value: count('stalled') },
    { name: '已暂停', value: count('paused') },
  ]

  return (
    <>
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        <StatCard
          title='抓取进程'
          value={overview ? (overview.worker_online ? `${overview.worker_count} 个在线` : '离线') : undefined}
          hint={
            overview?.worker_last_seen_at ? (
              <>
                最近心跳 <TimeAgo iso={overview.worker_last_seen_at} />
              </>
            ) : (
              '还没有心跳记录'
            )
          }
          icon={Server}
        />
        <StatCard title='待执行' value={queue ? count('queued') + count('stalled') : undefined} hint='已到调度时间' icon={Clock} />
        <StatCard title='抓取中' value={queue ? count('running') : undefined} hint='已被进程领取' icon={Activity} />
        <StatCard
          title='等待重试'
          value={queue ? count('retry') : undefined}
          hint='失败后按退避时间重试'
          icon={TriangleAlert}
        />
      </div>
      <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
        <Card className='col-span-1 lg:col-span-4'>
          <CardHeader>
            <CardTitle>连续失败最多的博客</CardTitle>
            <CardDescription>按连续失败次数排序，处理后次数会清零。</CardDescription>
          </CardHeader>
          <CardContent>
            {failing.length ? (
              <SimpleBarList
                items={failing.map((item) => ({ name: `${item.name || item.host}（${item.host}）`, value: item.consecutive_failures }))}
                barClass='bg-destructive'
                valueFormatter={(n) => `${n} 次`}
              />
            ) : (
              <p className='text-sm text-muted-foreground'>目前没有连续失败的博客。</p>
            )}
          </CardContent>
        </Card>
        <Card className='col-span-1 lg:col-span-3'>
          <CardHeader>
            <CardTitle>任务状态</CardTitle>
            <CardDescription>共 {items.length} 个抓取任务</CardDescription>
          </CardHeader>
          <CardContent>
            <SimpleBarList items={states} barClass='bg-muted-foreground' valueFormatter={(n) => `${n}`} />
          </CardContent>
        </Card>
      </div>
    </>
  )
}
