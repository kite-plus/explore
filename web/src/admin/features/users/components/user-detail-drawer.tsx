import { useState } from 'react'
import { Ban, Copy, MoreHorizontal, RotateCcw, TriangleAlert, Unlink } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/admin/lib/utils'
import { formatDateTime } from '@/admin/lib/format'
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
import { Skeleton } from '@/admin/components/ui/skeleton'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { QueryError } from '@/admin/components/query-error'
import { TimeAgo } from '@/admin/components/time-ago'
import { UserAvatar } from '@/admin/components/user-avatar'
import { Link } from '@/admin/router'
import type { AdminUserBlog, AdminUserDetail } from '@/lib/admin-types'
import { useUserDetail } from '../api'
import { reportStatuses, userRole, userStatus } from '../data/data'
import { UserActionsMenu } from './data-table-row-actions'
import { useIsSelf, useUsers } from './users-provider'

const linkClass = 'text-muted-foreground underline-offset-4 hover:text-foreground hover:underline'

function Section({ title, count, children }: { title: string; count?: number; children: React.ReactNode }) {
  return (
    <section className='space-y-3'>
      <h3 className='flex items-baseline gap-2 text-sm font-medium'>
        {title}
        {count !== undefined && <span className='text-muted-foreground tabular-nums'>{count}</span>}
      </h3>
      {children}
    </section>
  )
}

function Empty({ children }: { children: React.ReactNode }) {
  return <p className='text-sm text-muted-foreground'>{children}</p>
}

function BlogRow({ blog, since, action }: { blog: AdminUserBlog; since: string; action?: React.ReactNode }) {
  return (
    <li className='flex items-center gap-3'>
      <BlogAvatar host={blog.host} name={blog.name} className='size-7' />
      <div className='grid min-w-0 flex-1'>
        <Link to={`/admin/blogs?filter=${encodeURIComponent(blog.host)}`} className='truncate text-sm font-medium hover:underline'>
          {blog.name || blog.host}
        </Link>
        <span className='truncate font-mono text-xs text-muted-foreground'>{blog.host}</span>
      </div>
      {blog.status === 'paused' && (
        <Badge variant='outline' className='font-normal text-muted-foreground'>
          已暂停
        </Badge>
      )}
      <span className='text-xs text-nowrap text-muted-foreground' title={formatDateTime(blog.since)}>
        {since}
        <TimeAgo iso={blog.since} />
      </span>
      {action}
    </li>
  )
}

function Overview({ user }: { user: AdminUserDetail }) {
  const status = userStatus(user)
  const role = userRole(user)
  const lastSignIn = user.sessions[0]?.created_at

  return (
    <dl className='grid grid-cols-[6rem_1fr] gap-x-4 gap-y-2.5 text-sm'>
      <dt className='text-muted-foreground'>状态</dt>
      <dd className={cn('flex items-center gap-2', status.className)}>
        <status.icon className='size-4' />
        {status.label}
      </dd>
      <dt className='text-muted-foreground'>角色</dt>
      <dd className='flex items-center gap-2'>
        <role.icon className='size-4 text-muted-foreground' />
        {role.label}
      </dd>
      <dt className='text-muted-foreground'>用户 ID</dt>
      <dd className='flex min-w-0 items-center gap-1'>
        <span className='truncate font-mono text-xs'>{user.id}</span>
        <Button
          variant='ghost'
          size='icon'
          className='size-6 shrink-0'
          aria-label='复制用户 ID'
          onClick={() =>
            void navigator.clipboard.writeText(user.id).then(
              () => toast.success('已复制用户 ID'),
              () => toast.error('复制失败')
            )
          }
        >
          <Copy className='size-3.5' />
        </Button>
      </dd>
      <dt className='text-muted-foreground'>注册时间</dt>
      <dd>{formatDateTime(user.created_at)}</dd>
      <dt className='text-muted-foreground'>最近活跃</dt>
      <dd>{user.last_seen_at ? <TimeAgo iso={user.last_seen_at} /> : '从未'}</dd>
      <dt className='text-muted-foreground'>登录会话</dt>
      <dd>
        {user.sessions.length ? (
          <>
            {user.sessions.length} 个有效
            <span className='text-muted-foreground'>
              ，最近一次登录 <TimeAgo iso={lastSignIn} />
            </span>
          </>
        ) : (
          <span className='text-muted-foreground'>没有有效会话</span>
        )}
      </dd>
    </dl>
  )
}

function Details({ user }: { user: AdminUserDetail }) {
  const { act } = useUsers()
  const self = useIsSelf()(user)

  return (
    <>
      {user.disabled_at && (
        <Alert variant='destructive'>
          <TriangleAlert />
          <AlertTitle>账号已停用</AlertTitle>
          <AlertDescription>
            <p>{user.disabled_reason || '没有记录原因。'}</p>
            <p className='text-xs opacity-80'>
              {formatDateTime(user.disabled_at)}
              {user.disabled_by && ` · ${user.disabled_by}`}
            </p>
          </AlertDescription>
        </Alert>
      )}
      <Section title='概况'>
        <Overview user={user} />
      </Section>
      <Separator />
      <Section title='认领的博客' count={user.owned_blog_count}>
        {user.owned_blogs.length ? (
          <ul className='space-y-3'>
            {user.owned_blogs.map((blog) => (
              <BlogRow
                key={blog.host}
                blog={blog}
                since='认领于 '
                action={
                  <Button
                    variant='ghost'
                    size='icon'
                    className='size-7 shrink-0 text-muted-foreground hover:text-destructive'
                    aria-label={`解除 ${blog.host} 的认领`}
                    title='解除认领'
                    onClick={() => act('release', [user], { host: blog.host })}
                  >
                    <Unlink className='size-4' />
                  </Button>
                }
              />
            ))}
          </ul>
        ) : (
          <Empty>没有认领博客。</Empty>
        )}
        {user.pending_claims.length > 0 && (
          <p className='text-xs text-muted-foreground'>
            正在验证：
            {user.pending_claims.map((claim, index) => (
              <span key={claim.host}>
                {index > 0 && '、'}
                <span className='font-mono'>{claim.host}</span>（
                <TimeAgo iso={claim.expires_at} />
                过期）
              </span>
            ))}
          </p>
        )}
      </Section>
      <Separator />
      <Section title='订阅' count={user.subscription_count}>
        {user.subscriptions.length ? (
          <>
            <ul className='space-y-3'>
              {user.subscriptions.map((blog) => (
                <BlogRow key={blog.host} blog={blog} since='订阅于 ' />
              ))}
            </ul>
            {user.subscription_count > user.subscriptions.length && (
              <Empty>只列出最近的 {user.subscriptions.length} 个。</Empty>
            )}
          </>
        ) : (
          <Empty>还没有订阅博客。</Empty>
        )}
      </Section>
      <Separator />
      <Section title='提交的举报' count={user.reports.length}>
        {user.reports.length ? (
          <ul className='space-y-3'>
            {user.reports.map((report) => {
              const status = reportStatuses[report.status]
              return (
                <li key={report.id} className='space-y-1 text-sm'>
                  <div className='flex items-center gap-2'>
                    <Badge variant={status.variant} className='font-normal'>
                      {status.label}
                    </Badge>
                    <span className='min-w-0 flex-1 truncate font-medium'>
                      {report.target_type === 'entry' ? report.entry_title || '（文章已不在缓存中）' : report.blog_host}
                    </span>
                    <TimeAgo iso={report.created_at} className='text-xs text-nowrap text-muted-foreground' />
                  </div>
                  <p className='line-clamp-2 text-muted-foreground'>{report.reason}</p>
                </li>
              )
            })}
          </ul>
        ) : (
          <Empty>没有提交过举报。</Empty>
        )}
        {user.reports.length > 0 && (
          <Link to={`/admin/takedowns?filter=${encodeURIComponent(user.email)}`} className={cn('inline-block text-sm', linkClass)}>
            在下架审批里查看
          </Link>
        )}
      </Section>
      {self && <Empty>这是你自己的账号：停用、权限和强制下线不能在这里操作，密码请在个人资料页修改。</Empty>}
    </>
  )
}

function Loading() {
  return (
    <div className='space-y-4'>
      {Array.from({ length: 6 }, (_, index) => (
        <Skeleton key={index} className='h-5 w-full' />
      ))}
    </div>
  )
}

type UserDetailDrawerProps = {
  id: string | null
  onClose: () => void
}

export function UserDetailDrawer({ id, onClose }: UserDetailDrawerProps) {
  // The last account shown stays while the drawer animates out.
  const [shownId, setShownId] = useState(id)
  if (id && id !== shownId) setShownId(id)
  const { data: user, error, refetch } = useUserDetail(shownId)
  const { act } = useUsers()
  const self = useIsSelf()

  return (
    <Sheet open={Boolean(id)} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className='flex flex-col sm:max-w-lg'>
        <SheetHeader className='text-start'>
          {user ? (
            <div className='flex items-center gap-3'>
              <UserAvatar name={user.display_name} email={user.email} className='size-10 text-base' />
              <div className='min-w-0'>
                <SheetTitle className='truncate'>{user.display_name}</SheetTitle>
                <SheetDescription className='truncate'>{user.email}</SheetDescription>
              </div>
            </div>
          ) : (
            <>
              <SheetTitle>用户详情</SheetTitle>
              <SheetDescription className='sr-only'>账号的概况、认领、订阅和举报记录</SheetDescription>
            </>
          )}
        </SheetHeader>
        <div className='flex-1 space-y-6 overflow-y-auto px-4'>
          {error ? (
            <QueryError message={error.message} onRetry={() => void refetch()} />
          ) : user ? (
            <Details user={user} />
          ) : (
            <Loading />
          )}
        </div>
        {user && (
          <SheetFooter className='flex-row justify-end'>
            {!self(user) &&
              (user.disabled_at ? (
                <Button variant='outline' onClick={() => act('restore', [user])}>
                  <RotateCcw />
                  恢复账号
                </Button>
              ) : (
                <Button variant='outline' className='text-destructive hover:text-destructive' onClick={() => act('disable', [user])}>
                  <Ban />
                  停用账号
                </Button>
              ))}
            <UserActionsMenu user={user}>
              <Button variant='outline'>
                <MoreHorizontal />
                更多操作
              </Button>
            </UserActionsMenu>
          </SheetFooter>
        )}
      </SheetContent>
    </Sheet>
  )
}
