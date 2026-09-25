import { cn } from '@/admin/lib/utils'

/**
 * The centered page sign-in and setup share, after shadcn-admin's sign-in-2.
 * min-h-svh rather than h-svh, so a form taller than a phone's screen scrolls
 * instead of losing its top.
 */
export function AuthLayout({ children, wide = false }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <div className='relative container grid min-h-svh flex-col items-center justify-center lg:max-w-none lg:px-0'>
      <div className='lg:p-8'>
        <div className='mx-auto flex w-full flex-col justify-center space-y-2 py-8 sm:w-120 sm:p-8'>
          <div className='mb-4 flex items-center justify-center'>
            <img src='/favicon.svg' alt='' width={29} height={24} className='me-2 h-6 w-auto' />
            <h1 className='text-xl font-medium'>Explore 管理后台</h1>
          </div>
        </div>
        <div className={cn('mx-auto flex w-full flex-col justify-center gap-6 pb-8', wide ? 'max-w-md' : 'max-w-sm')}>
          {children}
        </div>
      </div>
    </div>
  )
}
