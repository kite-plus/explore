import { useState } from 'react'
import { cn } from '@/admin/lib/utils'
import { avatarColor, initial } from '@/admin/lib/format'
import { Avatar, AvatarFallback } from '@/admin/components/ui/avatar'

/**
 * A blog's favicon over the reader site's fallback, the first letter on a
 * color from the host. The letter stays when the API has no icon, which it
 * answers with a 1x1 transparent PNG, or 404s because the blog is hidden.
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
  // Keyed by host so an avatar reused for another blog drops the old icon.
  const [faviconHost, setFaviconHost] = useState<string | null>(null)
  return (
    <Avatar className={cn(large ? 'size-10' : 'size-8', className)} aria-hidden='true'>
      <AvatarFallback className={cn('text-sm font-medium text-white', avatarColor(host))}>
        {initial(name || host)}
      </AvatarFallback>
      {/* invisible rather than hidden: a lazy image without a box never loads. */}
      <img
        src={`/api/v1/blogs/${encodeURIComponent(host)}/favicon`}
        alt=''
        loading='lazy'
        decoding='async'
        onLoad={(event) => setFaviconHost(event.currentTarget.naturalWidth > 1 ? host : null)}
        className={cn(
          'absolute inset-0 size-full bg-white object-contain p-1',
          faviconHost !== host && 'invisible'
        )}
      />
    </Avatar>
  )
}
