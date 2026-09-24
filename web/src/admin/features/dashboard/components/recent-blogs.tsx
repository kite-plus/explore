import { Link } from '@/admin/router'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminBlog } from '@/lib/admin-types'

export function RecentBlogs({ blogs }: { blogs: AdminBlog[] }) {
  if (blogs.length === 0) {
    return <p className='text-sm text-muted-foreground'>还没有收录博客。</p>
  }
  return (
    <ul className='flex flex-col gap-1'>
      {blogs.map((blog) => (
        <li key={blog.host}>
          <Link
            to={`/admin/blogs?filter=${encodeURIComponent(blog.host)}`}
            className='-mx-2 flex items-center gap-3 rounded-md px-2 py-2 hover:bg-accent/50'
          >
            <BlogAvatar host={blog.host} name={blog.name} />
            <div className='min-w-0 flex-1'>
              <p className='truncate text-sm font-medium'>{blog.name || blog.host}</p>
              <p className='truncate font-mono text-xs text-muted-foreground'>{blog.host}</p>
            </div>
            <TimeAgo iso={blog.created_at} className='shrink-0 text-xs text-muted-foreground' />
          </Link>
        </li>
      ))}
    </ul>
  )
}
