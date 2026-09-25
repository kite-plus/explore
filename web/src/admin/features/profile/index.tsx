import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Loader2, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'
import { AdminRequestError } from '@/lib/admin-api'
import type { ReaderUser } from '@/lib/reader-api'
import { toastError, useAccountRequest } from '@/admin/lib/api'
import { useAuth } from '@/admin/context/auth-provider'
import { Badge } from '@/admin/components/ui/badge'
import { Button } from '@/admin/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/admin/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/admin/components/ui/form'
import { Input } from '@/admin/components/ui/input'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { PasswordInput } from '@/admin/components/password-input'
import { UserAvatar } from '@/admin/components/user-avatar'

const MAX_NAME = 80
const MIN_PASSWORD = 12
// The API counts bytes, as bcrypt reads no more than 72.
const MAX_PASSWORD_BYTES = 72

const nameSchema = z.object({
  display_name: z.string().trim().min(1, '请输入名称。').max(MAX_NAME, `名称最多 ${MAX_NAME} 个字。`),
})
type NameForm = z.infer<typeof nameSchema>

const passwordSchema = z
  .object({
    current_password: z.string().min(1, '请输入当前密码。'),
    new_password: z
      .string()
      .min(MIN_PASSWORD, `新密码至少 ${MIN_PASSWORD} 位。`)
      .refine(
        (value) => new TextEncoder().encode(value).length <= MAX_PASSWORD_BYTES,
        '新密码太长了，最多 72 个英文字符或 24 个汉字。'
      ),
    confirm: z.string(),
  })
  .refine((values) => values.confirm === values.new_password, {
    path: ['confirm'],
    message: '两次输入的新密码不一致。',
  })
type PasswordForm = z.infer<typeof passwordSchema>

export function Profile() {
  return (
    <>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='个人资料' description='你的名称和登录密码。' />
        <div className='grid gap-4 sm:gap-6 xl:grid-cols-2 xl:items-start'>
          <AccountCard />
          <PasswordCard />
        </div>
      </Main>
    </>
  )
}

function AccountCard() {
  const { user, signedIn } = useAuth()
  const request = useAccountRequest()
  const form = useForm<NameForm>({
    resolver: zodResolver(nameSchema),
    defaultValues: { display_name: user?.name ?? '' },
  })
  const save = useMutation({
    mutationFn: (values: NameForm) =>
      request<ReaderUser>('/me', { method: 'PATCH', body: JSON.stringify(values) }),
    onSuccess: (account) => {
      signedIn(account)
      form.reset({ display_name: account.display_name })
      toast.success('名称已更新')
    },
    onError: (cause) => toastError(cause, '保存失败，请重试。'),
  })
  if (!user) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle>基本信息</CardTitle>
        <CardDescription>名称显示在前台和后台；邮箱用于登录，不能修改。</CardDescription>
      </CardHeader>
      <Form {...form}>
        <form onSubmit={form.handleSubmit((values) => save.mutate(values))} className='flex flex-col gap-6'>
          <CardContent className='space-y-6'>
            <div className='flex items-center gap-4'>
              <UserAvatar name={user.name} email={user.email} className='size-14 text-xl' />
              <div className='grid min-w-0 gap-1'>
                <div className='flex min-w-0 items-center gap-2'>
                  <span className='truncate font-semibold'>{user.name}</span>
                  <Badge variant='secondary'>
                    <ShieldCheck />
                    管理员
                  </Badge>
                </div>
                <span className='truncate text-sm text-muted-foreground'>{user.email}</span>
              </div>
            </div>
            <FormField
              control={form.control}
              name='display_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='name' maxLength={MAX_NAME} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
          <CardFooter className='gap-2'>
            <Button type='submit' disabled={save.isPending || !form.formState.isDirty}>
              {save.isPending && <Loader2 className='animate-spin' />}
              保存
            </Button>
            <Button
              type='button'
              variant='outline'
              disabled={save.isPending || !form.formState.isDirty}
              onClick={() => form.reset({ display_name: user.name })}
            >
              撤销修改
            </Button>
          </CardFooter>
        </form>
      </Form>
    </Card>
  )
}

function PasswordCard() {
  const { user } = useAuth()
  const request = useAccountRequest()
  const form = useForm<PasswordForm>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { current_password: '', new_password: '', confirm: '' },
  })
  const change = useMutation({
    mutationFn: ({ current_password, new_password }: PasswordForm) =>
      request('/me/password', {
        method: 'PUT',
        body: JSON.stringify({ current_password, new_password }),
      }),
    onSuccess: () => {
      form.reset()
      toast.success('密码已修改', { description: '其他设备上的登录已失效，这台设备保持登录。' })
    },
    onError: (cause) => {
      if (cause instanceof AdminRequestError && cause.code === 'wrong_password') {
        form.setError('current_password', { message: '当前密码不正确。' }, { shouldFocus: true })
        return
      }
      toastError(cause, '修改失败，请重试。')
    },
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>修改密码</CardTitle>
        <CardDescription>修改后，其他设备上的登录会失效，这台设备保持登录。</CardDescription>
      </CardHeader>
      <Form {...form}>
        <form onSubmit={form.handleSubmit((values) => change.mutate(values))} className='flex flex-col gap-6'>
          {/* Lets password managers tell whose password changed. */}
          <input type='email' autoComplete='username' value={user?.email ?? ''} readOnly hidden />
          <CardContent className='space-y-4'>
            <FormField
              control={form.control}
              name='current_password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>当前密码</FormLabel>
                  <FormControl>
                    <PasswordInput {...field} autoComplete='current-password' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='new_password'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>新密码</FormLabel>
                  <FormControl>
                    <PasswordInput {...field} autoComplete='new-password' placeholder={`至少 ${MIN_PASSWORD} 位`} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='confirm'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>确认新密码</FormLabel>
                  <FormControl>
                    <PasswordInput {...field} autoComplete='new-password' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </CardContent>
          <CardFooter>
            <Button type='submit' disabled={change.isPending}>
              {change.isPending && <Loader2 className='animate-spin' />}
              修改密码
            </Button>
          </CardFooter>
        </form>
      </Form>
    </Card>
  )
}
