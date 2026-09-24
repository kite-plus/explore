import { ExternalLink, Pencil, RefreshCw, TriangleAlert } from 'lucide-react'
import { formatDateTime, formatInterval } from '@/admin/lib/format'
import { cn } from '@/admin/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { Badge } from '@/admin/components/ui/badge'
import { Button } from '@/admin/components/ui/button'
import { Separator } from '@/admin/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/admin/components/ui/sheet'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { GeneratorLabel } from '@/admin/components/generator-label'
import { TimeAgo } from '@/admin/components/time-ago'
import { Link } from '@/admin/router'
import type { AdminBlog } from '@/lib/admin-types'
import { useFetchNow } from '../api'
import { blogState, blogStates } from '../data/data'

type BlogDetailDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onEdit: () => void
  blog: AdminBlog
}

export function BlogDetailDrawer({ open, onOpenChange, onEdit, blog }: BlogDetailDrawerProps) {
  const fetchNow = useFetchNow()
  const state = blogStates.find((item) => item.value === blogState(blog))

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col sm:max-w-lg'>
        <SheetHeader className='text-start'>
          <div className='flex items-center gap-3'>
            <BlogAvatar host={blog.host} name={blog.name} large />
            <div className='min-w-0'>
              <SheetTitle className='truncate'>{blog.name || blog.host}</SheetTitle>
              <SheetDescription className='truncate font-mono'>{blog.host}</SheetDescription>
            </div>
          </div>
        </SheetHeader>
        <div className='flex-1 space-y-6 overflow-y-auto px-4'>
          {blog.gone_since && (
            <Alert variant='destructive'>
              <TriangleAlert />
              <AlertTitle>博客已失联</AlertTitle>
              <AlertDescription>
                <p>
                  <TimeAgo iso={blog.gone_since} />
                  被标记为失联（订阅源返回 410，或 robots.txt 明确拒绝抓取）。7 天内没有恢复会被自动移除。
                </p>
              </AlertDescription>
            </Alert>
          )}
          <section className='space-y-3'>
            <h3 className='text-sm font-medium'>概况</h3>
            <dl className='grid grid-cols-[6rem_1fr] gap-x-4 gap-y-2.5 text-sm'>
              <dt className='text-muted-foreground'>状态</dt>
              <dd className={cn('flex items-center gap-2', state?.className)}>
                {state && <state.icon className='size-4' />}
                {state?.label}
              </dd>
              {blog.status === 'paused' && blog.status_note && (
                <>
                  <dt className='text-muted-foreground'>暂停原因</dt>
                  <dd>{blog.status_note}</dd>
                </>
              )}
              <dt className='text-muted-foreground'>网站</dt>
              <dd className='break-all'>
                <a href={blog.site_url} target='_blank' rel='noopener' className='hover:underline'>
                  {blog.site_url}
                </a>
              </dd>
              <dt className='text-muted-foreground'>订阅源</dt>
              <dd className='break-all'>
                <a href={blog.feed_url} target='_blank' rel='noopener' className='hover:underline'>
                  {blog.feed_url}
                </a>
              </dd>
              <dt className='text-muted-foreground'>程序</dt>
              <dd>
                <GeneratorLabel value={blog.generator} />
              </dd>
              <dt className='text-muted-foreground'>语言</dt>
              <dd>{blog.language || '—'}</dd>
              <dt className='text-muted-foreground'>文章摘要</dt>
              <dd>{blog.show_excerpt ? '展示' : '不展示'}</dd>
              <dt className='text-muted-foreground'>额外域名</dt>
              <dd className='flex flex-wrap gap-1'>
                {blog.extra_domains?.length
                  ? blog.extra_domains.map((domain) => (
                      <Badge key={domain} variant='secondary' className='font-mono'>
                        {domain}
                      </Badge>
                    ))
                  : '—'}
              </dd>
              <dt className='text-muted-foreground'>默认标签</dt>
              <dd className='flex flex-wrap gap-1'>
                {blog.default_tags?.length
                  ? blog.default_tags.map((tag) => (
                      <Badge key={tag} variant='outline'>
                        {tag}
                      </Badge>
                    ))
                  : '—'}
              </dd>
              <dt className='text-muted-foreground'>收录时间</dt>
              <dd>{formatDateTime(blog.created_at)}</dd>
            </dl>
          </section>
          <Separator />
          <section className='space-y-3'>
            <h3 className='text-sm font-medium'>抓取</h3>
            <dl className='grid grid-cols-[6rem_1fr] gap-x-4 gap-y-2.5 text-sm'>
              <dt className='text-muted-foreground'>抓取间隔</dt>
              <dd>{formatInterval(blog.fetch_interval_seconds)}</dd>
              <dt className='text-muted-foreground'>下次抓取</dt>
              <dd>
                <TimeAgo iso={blog.next_fetch_at} />
              </dd>
              <dt className='text-muted-foreground'>上次抓取</dt>
              <dd>
                <TimeAgo iso={blog.last_fetched_at} />
              </dd>
              <dt className='text-muted-foreground'>上次成功</dt>
              <dd>
                <TimeAgo iso={blog.last_succeeded_at} />
              </dd>
              <dt className='text-muted-foreground'>连续失败</dt>
              <dd className={blog.consecutive_failures > 0 ? 'text-destructive' : undefined}>
                {blog.consecutive_failures} 次
              </dd>
            </dl>
            {blog.last_error && (
              <pre className='rounded-md bg-muted p-3 font-mono text-xs break-all whitespace-pre-wrap text-destructive'>
                {blog.last_error}
              </pre>
            )}
            <Link
              to={`/admin/queue?filter=${encodeURIComponent(blog.host)}`}
              className='inline-block text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline'
            >
              在抓取任务里查看执行记录
            </Link>
          </section>
        </div>
        <SheetFooter className='gap-2 sm:flex-row sm:justify-end'>
          <Button variant='outline' asChild>
            <a href={blog.site_url} target='_blank' rel='noopener'>
              <ExternalLink />
              访问博客
            </a>
          </Button>
          <Button
            variant='outline'
            disabled={blog.status === 'paused'}
            onClick={() => void fetchNow([blog.host])}
          >
            <RefreshCw />
            立即抓取
          </Button>
          <Button onClick={onEdit}>
            <Pencil />
            编辑
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
