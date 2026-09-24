import { cn } from '@/admin/lib/utils'
import { avatarColor, initial } from '@/admin/lib/format'
import { Avatar, AvatarFallback, AvatarImage } from '@/admin/components/ui/avatar'

/**
 * The reader site's blog avatar: the first letter on a color from the host.
 * Only the large one loads the favicon, as on the reader site, so a long
 * table does not request one favicon per row.
 */
export function BlogAvatar({
  host,
  name,
  large = false,
  className,
}: {
  host: string
  name: string
  large?: boolean
  className?: string
}) {
  return (
    <Avatar className={cn(large ? 'size-10' : 'size-8', className)} aria-hidden='true'>
      {large && (
        <AvatarImage
          src={`/api/v1/blogs/${encodeURIComponent(host)}/favicon`}
          alt=''
          className='bg-white object-contain p-1'
        />
      )}
      <AvatarFallback className={cn('text-sm font-medium text-white', avatarColor(host))}>
        {initial(name || host)}
      </AvatarFallback>
    </Avatar>
  )
}
