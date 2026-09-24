import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { toastError, useAdminMutation } from '@/admin/lib/api'
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
import type { AdminSubmission } from '@/lib/admin-types'
import { QUICK_REASONS } from '../data/data'

const formSchema = z.object({
  review_note: z.string().trim().min(1, '请填写驳回原因。').max(500, '驳回原因最多 500 字。'),
})
type RejectForm = z.infer<typeof formSchema>

type RejectDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  submission: AdminSubmission
}

export function RejectDialog({ open, onOpenChange, submission }: RejectDialogProps) {
  const reject = useAdminMutation((values: RejectForm) => ({
    path: `/submissions/${encodeURIComponent(submission.id)}/reject`,
    method: 'POST',
    body: values,
  }))
  const form = useForm<RejectForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { review_note: '' },
  })
  const note = form.watch('review_note')

  const onSubmit = async (values: RejectForm) => {
    try {
      await reject.mutateAsync(values)
      toast.success(`已驳回 ${submission.host}`)
      onOpenChange(false)
    } catch (cause) {
      toastError(cause)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !reject.isPending && onOpenChange(state)}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-start'>
          <DialogTitle>驳回 {submission.host}</DialogTitle>
          <DialogDescription>驳回原因会显示在提交者的进度页上。</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='reject-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <div className='flex flex-wrap gap-1.5'>
              {QUICK_REASONS.map((reason) => (
                <Button
                  key={reason}
                  type='button'
                  variant='outline'
                  size='sm'
                  className='h-7 rounded-full px-2.5 text-xs font-normal'
                  onClick={() => form.setValue('review_note', reason, { shouldValidate: true })}
                >
                  {reason}
                </Button>
              ))}
            </div>
            <FormField
              control={form.control}
              name='review_note'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>驳回原因</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={3} autoFocus placeholder='点击上方的常用原因，或自己填写' />
                  </FormControl>
                  <FormDescription className='tabular-nums'>{note.length} / 500</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='reject-form' variant='destructive' disabled={reject.isPending}>
            {reject.isPending && <Loader2 className='animate-spin' />}
            确认驳回
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
