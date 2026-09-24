import { Button } from '@/admin/components/ui/button'

/** Shown in place of a list or page when its data fails to load. */
export function QueryError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className='flex flex-col items-center gap-3 rounded-md border border-dashed p-10 text-center'>
      <p className='font-medium'>读取失败</p>
      <p className='text-sm text-muted-foreground'>{message}</p>
      <Button variant='outline' size='sm' onClick={onRetry}>
        重试
      </Button>
    </div>
  )
}
