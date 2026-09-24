import { Link } from '@/admin/router'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/admin/components/ui/card'
import type { AdminBlog } from '@/lib/admin-types'

/** How much of the directory readers see. Paused blogs are hidden on purpose. */
export function VisibleBlogs({ blogs }: { blogs: AdminBlog[] }) {
  const visible = blogs.filter((blog) => blog.visible).length
  const paused = blogs.filter((blog) => blog.status === 'paused').length
  const failing = blogs.length - visible - paused
  const hidden = [failing > 0 && `${failing} 个抓取异常`, paused > 0 && `${paused} 个已暂停`].filter(Boolean)

  return (
    <Card>
      <CardHeader>
        <CardTitle>读者可见</CardTitle>
        <CardDescription>
          {hidden.length ? `${hidden.join('、')}，暂不展示给读者。` : '所有博客都展示给读者。'}
        </CardDescription>
        {hidden.length > 0 && (
          <CardAction>
            <Link
              to={`/admin/blogs?status=${encodeURIComponent(JSON.stringify(['unhealthy', 'paused']))}`}
              className='text-sm text-muted-foreground hover:text-foreground'
            >
              查看
            </Link>
          </CardAction>
        )}
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        <div className='text-2xl font-semibold tabular-nums'>
          {visible.toLocaleString('zh-CN')}
          <span className='text-base font-normal text-muted-foreground'> / {blogs.length.toLocaleString('zh-CN')}</span>
        </div>
        {/* Width through the CSSOM, which the CSP allows. */}
        <div className='h-2 rounded-full bg-muted'>
          <div
            className='h-full rounded-full bg-success'
            style={{ width: `${blogs.length ? (visible / blogs.length) * 100 : 0}%` }}
          />
        </div>
      </CardContent>
    </Card>
  )
}
