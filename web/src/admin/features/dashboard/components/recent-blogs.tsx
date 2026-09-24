import { Link } from '@/admin/router'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminBlog } from '@/lib/admin-types'

export function RecentBlogs({ blogs }: { blogs: AdminBlog[] }) {
  if (blogs.length === 0) {
    return <p className='text-sm text-muted-foreground'>还没有收录博客。</p>
  }
  return (
    <div className='space-y-8'>
      {blogs.map((blog) => (
        <div key={blog.host} className='flex items-center gap-4'>
          <BlogAvatar host={blog.host} name={blog.name} className='h-9 w-9' />
          <div className='flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-2'>
            <div className='min-w-0 space-y-1'>
              <Link
                to={`/admin/blogs?filter=${encodeURIComponent(blog.host)}`}
                className='block truncate text-sm leading-none font-medium hover:underline'
              >
                {blog.name || blog.host}
              </Link>
              <p className='truncate font-mono text-sm text-muted-foreground'>{blog.host}</p>
            </div>
            <TimeAgo iso={blog.created_at} className='text-sm font-medium' />
          </div>
        </div>
      ))}
    </div>
  )
}
