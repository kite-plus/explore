import { cn } from '@/admin/lib/utils'
import { avatarColor, initial } from '@/admin/lib/format'
import { Avatar, AvatarFallback } from '@/admin/components/ui/avatar'

/**
 * A user's first letter on a color picked from their email, in the style of
 * the reader site's blog avatars. The color stays when the name changes.
 */
export function UserAvatar({
  name,
  email,
  className,
}: {
  name: string
  email: string
  className?: string
}) {
  return (
    <Avatar className={cn('text-sm', className)} aria-hidden='true'>
      <AvatarFallback className={cn('font-medium text-white', avatarColor(email))}>
        {initial(name || email)}
      </AvatarFallback>
    </Avatar>
  )
}
