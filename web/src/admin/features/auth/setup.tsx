import { useState } from 'react'
import { ArrowLeft, ArrowRight, Check, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { cn } from '@/admin/lib/utils'
import { useAuth } from '@/admin/context/auth-provider'
import { Button } from '@/admin/components/ui/button'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import { Switch } from '@/admin/components/ui/switch'
import { PasswordInput } from '@/admin/components/password-input'
import type { ReaderUser } from '@/lib/reader-api'
import { AuthLayout } from './auth-layout'

const STEPS = ['安装码', '管理员账号', '站点设置']
const MIN_PASSWORD = 12
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

class SetupError extends Error {
  constructor(
    readonly code: string,
    message: string
  ) {
    super(message)
  }
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`/api/v1/${path}`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', 'Accept-Language': 'zh-CN' },
    body: JSON.stringify(body),
  })
  if (response.ok) return (response.status === 204 ? undefined : await response.json()) as T
  const data = await response.json().catch(() => null)
  throw new SetupError(data?.error?.code ?? 'internal', data?.error?.message ?? `请求失败（HTTP ${response.status}）。`)
}

type Errors = Partial<Record<'code' | 'name' | 'email' | 'password' | 'confirm', string>>

/**
 * The first run of a fresh install: prove access to the server with the code
 * serve logs, create the first admin account, then choose the site switches.
 */
export function Setup() {
  const { signedIn } = useAuth()
  const [step, setStep] = useState(0)
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [registration, setRegistration] = useState(true)
  const [submissions, setSubmissions] = useState(true)
  const [checked, setChecked] = useState<number[]>([])
  const [serverErrors, setServerErrors] = useState<Errors>({})
  const [busy, setBusy] = useState(false)

  const errors: Errors = { ...(checked.includes(step) ? validate(step) : {}), ...serverErrors }

  function validate(at: number): Errors {
    const found: Errors = {}
    if (at === 0 && code.replace(/[^a-z0-9]/gi, '').length !== 12) found.code = '安装码是 12 位字母和数字。'
    if (at === 1) {
      if (!name.trim()) found.name = '请输入名称。'
      if (!EMAIL.test(email.trim())) found.email = '请输入有效的邮箱。'
      if (password.length < MIN_PASSWORD) found.password = `密码至少 ${MIN_PASSWORD} 位。`
      else if (confirm !== password) found.confirm = '两次输入的密码不一致。'
    }
    return found
  }

  function edited(field: keyof Errors) {
    if (serverErrors[field]) setServerErrors((current) => ({ ...current, [field]: undefined }))
  }

  function failed(cause: unknown) {
    if (!(cause instanceof SetupError)) {
      toast.error('无法连接服务器，请重试。')
      return
    }
    switch (cause.code) {
      case 'already_set_up':
        // Someone finished setup already; the sign-in page takes over.
        window.location.reload()
        return
      case 'invalid_setup_code':
        setStep(0)
        setServerErrors({ code: cause.message })
        return
      case 'conflict':
        setStep(1)
        setServerErrors({ email: '这个邮箱已经注册过了，请换一个。' })
        return
      default:
        toast.error(cause.message)
    }
  }

  async function next() {
    setChecked((current) => [...new Set([...current, step])])
    if (busy || Object.keys(validate(step)).length > 0 || Object.values(serverErrors).some(Boolean)) return
    if (step === 0) {
      setBusy(true)
      try {
        await post('setup/verify', { code })
        setStep(1)
      } catch (cause) {
        failed(cause)
      } finally {
        setBusy(false)
      }
      return
    }
    if (step === 1) {
      setStep(2)
      return
    }
    setBusy(true)
    try {
      const account = await post<ReaderUser>('setup', {
        code,
        email: email.trim(),
        password,
        display_name: name.trim(),
        registration_enabled: registration,
        submissions_enabled: submissions,
      })
      signedIn(account)
      toast.success('安装完成，欢迎使用 Explore！')
    } catch (cause) {
      failed(cause)
    } finally {
      setBusy(false)
    }
  }

  function onSubmit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault()
    void next()
  }

  return (
    <AuthLayout wide>
      <div className='flex flex-col space-y-2 text-start'>
        <h2 className='text-lg font-semibold tracking-tight'>安装 Explore</h2>
        <p className='text-sm text-muted-foreground'>三步完成安装，创建的账号就是这个站点的管理员。</p>
      </div>

      <ol className='flex flex-wrap items-center gap-x-3 gap-y-2 py-2 text-sm'>
        {STEPS.map((label, index) => (
          <li key={label} className='flex items-center gap-2'>
            <span
              className={cn(
                'flex size-6 shrink-0 items-center justify-center rounded-full border text-xs font-medium',
                index < step && 'border-success text-success',
                index === step && 'border-foreground bg-foreground text-background',
                index > step && 'text-muted-foreground'
              )}
            >
              {index < step ? <Check className='size-3.5' /> : index + 1}
            </span>
            <span className={cn(index === step ? 'font-medium' : 'text-muted-foreground')}>{label}</span>
            {index < STEPS.length - 1 && <span aria-hidden='true' className='h-px w-6 bg-border' />}
          </li>
        ))}
      </ol>

      <form onSubmit={onSubmit} noValidate className='grid gap-4'>
        {step === 0 && (
          <>
            <p className='text-sm text-muted-foreground'>
              为了确认你是这台服务器的部署者，请输入安装码。安装码打印在 serve 服务的日志里：
            </p>
            <pre className='rounded-md bg-muted px-3 py-2 font-mono text-xs break-all whitespace-pre-wrap'>
              docker compose logs serve | grep setup_code
            </pre>
            <Field id='setup-code' label='安装码' error={errors.code}>
              <Input
                id='setup-code'
                autoComplete='off'
                spellCheck={false}
                placeholder='XXXX-XXXX-XXXX'
                className='font-mono uppercase'
                value={code}
                onChange={(event) => {
                  setCode(event.target.value)
                  edited('code')
                }}
                aria-invalid={Boolean(errors.code)}
              />
            </Field>
          </>
        )}

        {step === 1 && (
          <>
            <Field id='setup-name' label='名称' error={errors.name}>
              <Input
                id='setup-name'
                autoComplete='name'
                placeholder='显示在后台和前台的名字'
                value={name}
                onChange={(event) => setName(event.target.value)}
                aria-invalid={Boolean(errors.name)}
              />
            </Field>
            <Field id='setup-email' label='邮箱' error={errors.email}>
              <Input
                id='setup-email'
                type='email'
                autoComplete='username'
                placeholder='name@example.com'
                value={email}
                onChange={(event) => {
                  setEmail(event.target.value)
                  edited('email')
                }}
                aria-invalid={Boolean(errors.email)}
              />
            </Field>
            <Field id='setup-password' label='密码' error={errors.password} hint={`至少 ${MIN_PASSWORD} 位，之后用它登录后台。`}>
              <PasswordInput
                id='setup-password'
                autoComplete='new-password'
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                aria-invalid={Boolean(errors.password)}
              />
            </Field>
            <Field id='setup-confirm' label='确认密码' error={errors.confirm}>
              <PasswordInput
                id='setup-confirm'
                autoComplete='new-password'
                value={confirm}
                onChange={(event) => setConfirm(event.target.value)}
                aria-invalid={Boolean(errors.confirm)}
              />
            </Field>
          </>
        )}

        {step === 2 && (
          <>
            <SwitchRow
              id='setup-registration'
              label='开放注册'
              description='读者可以注册账号，用来订阅博客和举报问题。'
              checked={registration}
              onCheckedChange={setRegistration}
            />
            <SwitchRow
              id='setup-submissions'
              label='开放投稿'
              description='任何人都可以提交博客，等你在后台审核。'
              checked={submissions}
              onCheckedChange={setSubmissions}
            />
            <p className='text-sm text-muted-foreground'>之后可以在「系统设置」里随时修改。</p>
          </>
        )}

        <div className='mt-2 flex gap-2 *:flex-1'>
          {step > 0 && (
            <Button type='button' variant='outline' disabled={busy} onClick={() => setStep(step - 1)}>
              <ArrowLeft />
              上一步
            </Button>
          )}
          <Button disabled={busy}>
            {busy ? <Loader2 className='animate-spin' /> : step === STEPS.length - 1 ? <Check /> : <ArrowRight />}
            {step === STEPS.length - 1 ? '完成安装' : '下一步'}
          </Button>
        </div>
      </form>
    </AuthLayout>
  )
}

function Field({
  id,
  label,
  error,
  hint,
  children,
}: {
  id: string
  label: string
  error?: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <div data-slot='form-item' className='grid gap-2'>
      <Label htmlFor={id} data-error={Boolean(error)} className='data-[error=true]:text-destructive'>
        {label}
      </Label>
      {children}
      {error ? (
        <p className='text-sm text-destructive'>{error}</p>
      ) : (
        hint && <p className='text-sm text-muted-foreground'>{hint}</p>
      )}
    </div>
  )
}

function SwitchRow({
  id,
  label,
  description,
  checked,
  onCheckedChange,
}: {
  id: string
  label: string
  description: string
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
      <div className='space-y-0.5'>
        <Label htmlFor={id} className='text-base'>
          {label}
        </Label>
        <p className='text-sm text-muted-foreground'>{description}</p>
      </div>
      <Switch id={id} checked={checked} onCheckedChange={onCheckedChange} />
    </div>
  )
}
