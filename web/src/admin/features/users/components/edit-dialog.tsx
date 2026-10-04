import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { AdminRequestError } from '@/lib/admin-api'
import { toastError, useAdminRequest } from '@/admin/lib/api'
import { Button } from '@/admin/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/admin/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/admin/components/ui/form'
import { Input } from '@/admin/components/ui/input'
import type { AdminUserRow } from '@/lib/admin-types'

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

const formSchema = z.object({
  name: z.string().trim().min(1, '请填写名称。').max(80, '名称最多 80 个字符。'),
  email: z.string().trim().toLowerCase().max(254, '邮箱太长了。').regex(EMAIL, '邮箱格式不正确。'),
})
type EditForm = z.infer<typeof formSchema>

type EditDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: AdminUserRow
  onDone: () => void
}

export function EditDialog({ open, onOpenChange, user, onDone }: EditDialogProps) {
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  const form = useForm<EditForm>({
    resolver: zodResolver(formSchema),
    values: { name: user.display_name, email: user.email },
  })
  const busy = form.formState.isSubmitting

  const onSubmit = async (values: EditForm) => {
    // Send only what changed, so an unchanged email is never checked again.
    const profile = {
      ...(values.name !== user.display_name && { display_name: values.name }),
      ...(values.email !== user.email && { email: values.email }),
    }
    if (Object.keys(profile).length === 0) return onDone()
    try {
      await request(`/users/${encodeURIComponent(user.id)}`, { method: 'PATCH', body: JSON.stringify(profile) })
      await queryClient.invalidateQueries({ queryKey: ['admin'] })
      toast.success(`${values.email}：资料已更新`)
      onDone()
    } catch (cause) {
      // A taken email belongs next to the field, where it can be fixed.
      if (cause instanceof AdminRequestError && cause.code === 'email_taken')
        form.setError('email', { message: cause.message })
      else toastError(cause)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !busy && onOpenChange(state)}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-start'>
          <DialogTitle>编辑资料</DialogTitle>
          <DialogDescription>
            ID {user.number}。用来处理用户联系你们提出的修改，例如填错的邮箱或不当的名称。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='edit-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='email'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>邮箱</FormLabel>
                  <FormControl>
                    <Input {...field} type='email' autoComplete='off' />
                  </FormControl>
                  <FormDescription>邮箱就是登录名，改了以后用户要用新邮箱登录，密码不变。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='edit-form' disabled={busy}>
            {busy && <Loader2 className='animate-spin' />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
