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
import type { AdminEntryRow } from '@/lib/admin-types'
import { useSetHidden } from '../api'

const formSchema = z.object({
  reason: z.string().trim().min(1, '请填写处理原因。').max(500, '处理原因最多 500 字。'),
})
type HideForm = z.infer<typeof formSchema>

type HideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  entries: AdminEntryRow[]
  onDone?: () => void
}

export function HideDialog({ open, onOpenChange, entries, onDone }: HideDialogProps) {
  const setHidden = useSetHidden()
  const form = useForm<HideForm>({ resolver: zodResolver(formSchema), defaultValues: { reason: '' } })
  const busy = form.formState.isSubmitting

  const onSubmit = async (values: HideForm) => {
    if (await setHidden(entries, true, values.reason)) {
      form.reset()
      onOpenChange(false)
      onDone?.()
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
          <DialogTitle>{entries.length === 1 ? '隐藏文章' : `隐藏 ${entries.length} 篇文章`}</DialogTitle>
          <DialogDescription className='line-clamp-2'>
            {entries.length === 1 ? entries[0].title : '选中的文章会一起隐藏，使用同一个原因。'}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='hide-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='reason'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>处理原因</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={3} autoFocus placeholder='记录依据，方便以后排查' />
                  </FormControl>
                  <FormDescription>
                    仅后台可见。文章会从公开页面、订阅流和图片代理中移除，重新抓取后仍然隐藏。
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='hide-form' variant='destructive' disabled={busy}>
            {busy && <Loader2 className='animate-spin' />}
            隐藏
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
