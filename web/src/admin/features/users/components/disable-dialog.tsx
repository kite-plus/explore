import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import { Badge } from '@/admin/components/ui/badge'
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
import { Textarea } from '@/admin/components/ui/textarea'
import type { AdminUserRow } from '@/lib/admin-types'
import { useUserActions } from '../api'

const formSchema = z.object({
  reason: z.string().trim().min(1, '请填写停用原因。').max(500, '停用原因最多 500 字。'),
})
type DisableForm = z.infer<typeof formSchema>

const presets = ['垃圾或批量注册账号', '滥用举报', '冒充他人', '账号主人要求']

type DisableDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  users: AdminUserRow[]
  onDone: () => void
}

export function DisableDialog({ open, onOpenChange, users, onDone }: DisableDialogProps) {
  const { setDisabled } = useUserActions()
  const form = useForm<DisableForm>({ resolver: zodResolver(formSchema), defaultValues: { reason: '' } })
  const busy = form.formState.isSubmitting

  const onSubmit = async (values: DisableForm) => {
    if (await setDisabled(users, true, values.reason)) {
      form.reset()
      onDone()
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (busy) return
        form.reset()
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-start'>
          <DialogTitle>{users.length === 1 ? '停用账号' : `停用 ${users.length} 个账号`}</DialogTitle>
          <DialogDescription>
            {users.length === 1 ? `${users[0].display_name}（${users[0].email}）` : '选中的账号会一起停用，使用同一个原因。'}
            <br />
            停用后不能登录，现有会话立即失效；订阅和认领记录会保留，可以随时恢复。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='disable-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='reason'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>停用原因</FormLabel>
                  <div className='flex flex-wrap gap-1.5'>
                    {presets.map((preset) => (
                      <Badge
                        key={preset}
                        asChild
                        variant={field.value === preset ? 'default' : 'outline'}
                        className={cn('cursor-pointer font-normal', field.value !== preset && 'hover:bg-accent')}
                      >
                        <button type='button' onClick={() => form.setValue('reason', preset, { shouldValidate: true })}>
                          {preset}
                        </button>
                      </Badge>
                    ))}
                  </div>
                  <FormControl>
                    <Textarea {...field} rows={3} autoFocus placeholder='记录依据，方便以后排查' />
                  </FormControl>
                  <FormDescription>仅后台可见，会和操作人一起记录。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='disable-form' variant='destructive' disabled={busy}>
            {busy && <Loader2 className='animate-spin' />}
            停用
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
