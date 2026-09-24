import { useNavigate } from '@/admin/router'
import { Button } from '@/admin/components/ui/button'

export function NotFoundError() {
  const navigate = useNavigate()
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>404</h1>
        <span className='font-medium'>页面不存在</span>
        <p className='text-center text-muted-foreground'>
          你要找的后台页面不存在， <br />
          可能已经移动或删除。
        </p>
        <div className='mt-6 flex gap-4'>
          <Button variant='outline' onClick={() => window.history.back()}>
            返回上一页
          </Button>
          <Button onClick={() => navigate({ to: '/admin' })}>回到工作台</Button>
        </div>
      </div>
    </div>
  )
}
