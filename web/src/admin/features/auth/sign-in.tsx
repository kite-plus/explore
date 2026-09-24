// From shadcn-admin's src/features/auth/sign-in/sign-in-2.tsx and its
// components/user-auth-form.tsx, without the screenshot panel so the form
// sits in the middle. There are no social sign-in buttons and no
// forgot-password link: admins are Explore accounts, and Explore has no
// password reset.
import { useEffect, useState } from 'react'
import { Loader2, LogIn } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '@/admin/context/auth-provider'
import { Button } from '@/admin/components/ui/button'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import { PasswordInput } from '@/admin/components/password-input'
import { readerRequest, type ReaderUser } from '@/lib/reader-api'
import { AuthLayout } from './auth-layout'

export function SignIn() {
  const { error } = useAuth()

  useEffect(() => {
    if (error) toast.error(error)
  }, [error])

  return (
    <AuthLayout>
      <div className='flex flex-col space-y-2 text-start'>
        <h2 className='text-lg font-semibold tracking-tight'>登录</h2>
        <p className='text-sm text-muted-foreground'>
          输入管理员账号的邮箱和密码。
          <br className='max-sm:hidden' />
          不是管理员？{' '}
          <a href='/' className='text-nowrap underline underline-offset-4 hover:text-primary'>
            返回首页
          </a>
        </p>
      </div>
      <UserAuthForm />
      <p className='px-8 text-center text-sm text-muted-foreground'>
        此页面仅供 Explore 管理员使用。想收录博客，请前往{' '}
        <a href='/submit' className='underline underline-offset-4 hover:text-primary'>
          提交页面
        </a>
        。
      </p>
    </AuthLayout>
  )
}

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function validate(email: string, password: string) {
  const errors: { email?: string; password?: string } = {}
  if (!email.trim()) errors.email = '请输入邮箱。'
  else if (!EMAIL.test(email.trim())) errors.email = '邮箱格式不正确。'
  if (!password) errors.password = '请输入密码。'
  return errors
}

function UserAuthForm() {
  const { signedIn } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  // Like react-hook-form in the original: check on submit, then on every change.
  const errors = submitted ? validate(email, password) : {}

  async function signIn(): Promise<string> {
    const account = await readerRequest<ReaderUser>('auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: email.trim(), password }),
    })
    if (!account.is_admin) {
      await readerRequest('auth/logout', { method: 'POST' }, account.csrf_token)
      throw new Error('这个账号没有后台权限。')
    }
    signedIn(account)
    return account.display_name
  }

  async function onSubmit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitted(true)
    if (isLoading || Object.keys(validate(email, password)).length > 0) return
    setIsLoading(true)
    const attempt = signIn()
    toast.promise(attempt, {
      loading: '正在登录…',
      success: (name) => `欢迎回来，${name}！`,
      error: (cause) => (cause instanceof Error ? cause.message : '登录失败，请重试。'),
    })
    try {
      await attempt
    } catch {
      // The toast already shows the reason.
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate className='grid gap-3'>
      <div data-slot='form-item' className='grid gap-2'>
        <Label htmlFor='admin-email' data-error={Boolean(errors.email)} className='data-[error=true]:text-destructive'>
          邮箱
        </Label>
        <Input
          id='admin-email'
          type='email'
          autoComplete='username'
          placeholder='name@example.com'
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          aria-invalid={Boolean(errors.email)}
          aria-describedby={errors.email ? 'admin-email-message' : undefined}
        />
        {errors.email && (
          <p id='admin-email-message' className='text-sm text-destructive'>
            {errors.email}
          </p>
        )}
      </div>
      <div data-slot='form-item' className='relative grid gap-2'>
        <Label
          htmlFor='admin-password'
          data-error={Boolean(errors.password)}
          className='data-[error=true]:text-destructive'
        >
          密码
        </Label>
        <PasswordInput
          id='admin-password'
          autoComplete='current-password'
          placeholder='********'
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          aria-invalid={Boolean(errors.password)}
          aria-describedby={errors.password ? 'admin-password-message' : undefined}
        />
        {errors.password && (
          <p id='admin-password-message' className='text-sm text-destructive'>
            {errors.password}
          </p>
        )}
      </div>
      <Button className='mt-2' disabled={isLoading}>
        {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
        登录
      </Button>
    </form>
  )
}
