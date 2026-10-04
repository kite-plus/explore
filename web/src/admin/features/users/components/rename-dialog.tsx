import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/admin/components/ui/dialog'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/admin/components/ui/form'
import { Input } from '@/admin/components/ui/input'
import type { AdminUserRow } from '@/lib/admin-types'
import { useUserActions } from '../api'

const formSchema = z.object({
  name: z.string().trim().min(1, '请填写名称。').max(80, '名称最多 80 个字符。'),
})
type RenameForm = z.infer<typeof formSchema>

type RenameDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: AdminUserRow
  onDone: () => void
}

export function RenameDialog({ open, onOpenChange, user, onDone }: RenameDialogProps) {
  const { rename } = useUserActions()
  const form = useForm<RenameForm>({ resolver: zodResolver(formSchema), values: { name: user.display_name } })
  const busy = form.formState.isSubmitting

  const onSubmit = async (values: RenameForm) => {
    if (values.name === user.display_name) return onDone()
    if (await rename(user, values.name)) onDone()
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !busy && onOpenChange(state)}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-start'>
          <DialogTitle>修改名称</DialogTitle>
          <DialogDescription>
            {user.email}
            <br />
            用来处理不当或冒用他人的名称，账号主人之后仍可自己修改。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='rename-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input {...field} autoFocus autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='rename-form' disabled={busy}>
            {busy && <Loader2 className='animate-spin' />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
