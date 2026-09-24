// From shadcn-admin's src/features/auth/sign-in/sign-in-2.tsx and its
// components/user-auth-form.tsx, without the screenshot panel so the form
// sits in the middle. The social sign-in buttons became the switch to
// maintainer tokens, and there is no forgot-password link because Explore
// has no password reset.
import { useEffect, useState } from 'react'
import { KeyRound, Loader2, LogIn, Mail } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '@/admin/context/auth-provider'
import { Button } from '@/admin/components/ui/button'
import { Checkbox } from '@/admin/components/ui/checkbox'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import { PasswordInput } from '@/admin/components/password-input'
import { ACCOUNT_ADMIN } from '@/lib/admin-api'
import { readerRequest, type ReaderUser } from '@/lib/reader-api'

type Mode = 'account' | 'token'

export function SignIn() {
  const { error } = useAuth()
  const [mode, setMode] = useState<Mode>('account')

  useEffect(() => {
    if (error) toast.error(error)
  }, [error])

  return (
    <div className='relative container grid h-svh flex-col items-center justify-center lg:max-w-none lg:px-0'>
      <div className='lg:p-8'>
        <div className='mx-auto flex w-full flex-col justify-center space-y-2 py-8 sm:w-120 sm:p-8'>
          <div className='mb-4 flex items-center justify-center'>
            <img src='/favicon.svg' alt='' width={29} height={24} className='me-2 h-6 w-auto' />
            <h1 className='text-xl font-medium'>Explore 管理后台</h1>
          </div>
        </div>
        <div className='mx-auto flex w-full max-w-sm flex-col justify-center space-y-2'>
          <div className='flex flex-col space-y-2 text-start'>
            <h2 className='text-lg font-semibold tracking-tight'>登录</h2>
            <p className='text-sm text-muted-foreground'>
              {mode === 'account' ? (
                <>
                  输入邮箱和密码，登录 Explore 管理后台。
                  <br className='max-sm:hidden' />
                  不是维护者？{' '}
                  <a href='/' className='text-nowrap underline underline-offset-4 hover:text-primary'>
                    返回首页
                  </a>
                </>
              ) : (
                <>
                  输入部署时配置的维护者令牌。
                  <br className='max-sm:hidden' />
                  令牌只保存在这个浏览器里。
                </>
              )}
            </p>
          </div>
          <UserAuthForm mode={mode} onModeChange={setMode} />
          <p className='px-8 text-center text-sm text-muted-foreground'>
            此页面仅供 Explore 维护者使用。想收录博客，请前往{' '}
            <a href='/submit' className='underline underline-offset-4 hover:text-primary'>
              提交页面
            </a>
            。
          </p>
        </div>
      </div>
    </div>
  )
}

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function validate(mode: Mode, email: string, password: string, token: string) {
  const errors: { email?: string; password?: string; token?: string } = {}
  if (mode === 'token') {
    if (!token.trim()) errors.token = '请输入维护者令牌。'
    return errors
  }
  if (!email.trim()) errors.email = '请输入邮箱。'
  else if (!EMAIL.test(email.trim())) errors.email = '邮箱格式不正确。'
  if (!password) errors.password = '请输入密码。'
  return errors
}

function UserAuthForm({ mode, onModeChange }: { mode: Mode; onModeChange: (mode: Mode) => void }) {
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [token, setToken] = useState('')
  const [remember, setRemember] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  // Like react-hook-form in the original: check on submit, then on every change.
  const errors = submitted ? validate(mode, email, password, token) : {}

  async function signIn(): Promise<string> {
    if (mode === 'token') {
      const value = token.trim()
      const response = await fetch('/api/v1/admin/session', {
        headers: { Authorization: `Bearer ${value}` },
      })
      if (!response.ok) {
        throw new Error(
          response.status === 401 ? '维护者令牌无效。' : `无法连接后台（HTTP ${response.status}）。`
        )
      }
      login(value, remember)
      return '维护者'
    }
    const user = await readerRequest<ReaderUser>('auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: email.trim(), password }),
    })
    if (!user.is_admin) {
      await readerRequest('auth/logout', { method: 'POST' }, user.csrf_token)
      throw new Error('这个账号没有后台权限。')
    }
    login(ACCOUNT_ADMIN, false)
    return user.display_name
  }

  async function onSubmit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    setSubmitted(true)
    if (isLoading || Object.keys(validate(mode, email, password, token)).length > 0) return
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

  function switchMode() {
    setSubmitted(false)
    onModeChange(mode === 'account' ? 'token' : 'account')
  }

  return (
    <form onSubmit={onSubmit} noValidate className='grid gap-3'>
      {mode === 'account' ? (
        <>
          <div data-slot='form-item' className='grid gap-2'>
            <Label
              htmlFor='admin-email'
              data-error={Boolean(errors.email)}
              className='data-[error=true]:text-destructive'
            >
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
        </>
      ) : (
        <>
          <div data-slot='form-item' className='grid gap-2'>
            <Label
              htmlFor='admin-token'
              data-error={Boolean(errors.token)}
              className='data-[error=true]:text-destructive'
            >
              维护者令牌
            </Label>
            <PasswordInput
              id='admin-token'
              autoComplete='off'
              spellCheck={false}
              placeholder='********'
              value={token}
              onChange={(event) => setToken(event.target.value)}
              aria-invalid={Boolean(errors.token)}
              aria-describedby={errors.token ? 'admin-token-message' : undefined}
            />
            {errors.token && (
              <p id='admin-token-message' className='text-sm text-destructive'>
                {errors.token}
              </p>
            )}
          </div>
          <div className='flex items-center gap-2'>
            <Checkbox
              id='admin-remember'
              checked={remember}
              onCheckedChange={(checked) => setRemember(checked === true)}
            />
            <Label htmlFor='admin-remember' className='font-normal'>
              在这台设备上记住令牌
            </Label>
          </div>
        </>
      )}
      <Button className='mt-2' disabled={isLoading}>
        {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
        登录
      </Button>

      <div className='relative my-2'>
        <div className='absolute inset-0 flex items-center'>
          <span className='w-full border-t' />
        </div>
        <div className='relative flex justify-center text-xs uppercase'>
          <span className='bg-background px-2 text-muted-foreground'>或者使用</span>
        </div>
      </div>

      <Button variant='outline' type='button' disabled={isLoading} onClick={switchMode}>
        {mode === 'account' ? <KeyRound className='h-4 w-4' /> : <Mail className='h-4 w-4' />}
        {mode === 'account' ? '维护者令牌' : '邮箱和密码'}
      </Button>
    </form>
  )
}
