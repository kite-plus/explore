import { cn } from '@/admin/lib/utils'
import { Button } from '@/admin/components/ui/button'

type GeneralErrorProps = React.HTMLAttributes<HTMLDivElement> & {
  minimal?: boolean
}

export function GeneralError({ className, minimal = false }: GeneralErrorProps) {
  return (
    <div className={cn('h-svh w-full', className)}>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        {!minimal && <h1 className='text-[7rem] leading-tight font-bold'>500</h1>}
        <span className='font-medium'>页面出错了</span>
        <p className='text-center text-muted-foreground'>
          这个页面遇到了意外错误。 <br /> 刷新后再试一次。
        </p>
        {!minimal && (
          <div className='mt-6 flex gap-4'>
            <Button variant='outline' onClick={() => window.history.back()}>
              返回上一页
            </Button>
            <Button onClick={() => window.location.reload()}>刷新页面</Button>
          </div>
        )}
      </div>
    </div>
  )
}
