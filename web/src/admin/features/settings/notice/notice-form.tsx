import { useEffect } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { formatDateTime } from '@/admin/lib/format'
import { toastError, useAdminMutation } from '@/admin/lib/api'
import { Button } from '@/admin/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/admin/components/ui/form'
import { Skeleton } from '@/admin/components/ui/skeleton'
import { Textarea } from '@/admin/components/ui/textarea'
import { QueryError } from '@/admin/components/query-error'
import { useSettings } from '../settings-api'

const MAX = 280

const formSchema = z.object({
  site_notice: z.string().trim().max(MAX, `公告最多 ${MAX} 字。`),
})
type NoticeFormValues = z.infer<typeof formSchema>

export function NoticeForm() {
  const { byKey, isPending, error, refetch } = useSettings()
  const saved = byKey.get('site_notice')
  const save = useAdminMutation((value: string) => ({
    path: '/settings/site_notice',
    method: 'PATCH',
    body: { value },
  }))
  const form = useForm<NoticeFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { site_notice: '' },
  })
  const notice = form.watch('site_notice')

  useEffect(() => {
    if (saved) form.reset({ site_notice: saved.value })
  }, [saved?.value])

  async function onSubmit(values: NoticeFormValues) {
    try {
      await save.mutateAsync(values.site_notice)
      toast.success(values.site_notice ? '公告已更新' : '公告已清除')
    } catch (cause) {
      toastError(cause, '保存失败，请重试。')
    }
  }

  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />
  if (isPending) return <Skeleton className='h-40 w-full' />

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <FormField
          control={form.control}
          name='site_notice'
          render={({ field }) => (
            <FormItem>
              <FormLabel>公告内容</FormLabel>
              <FormControl>
                <Textarea {...field} rows={4} placeholder='例如：今晚 23:00 起维护约 10 分钟。' />
              </FormControl>
              <FormDescription className='flex justify-between gap-4'>
                <span>
                  {saved ? `${saved.updated_by} 更新于 ${formatDateTime(saved.updated_at)}` : ''}
                </span>
                <span className='tabular-nums'>
                  {notice.length} / {MAX}
                </span>
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <div className='flex gap-2'>
          <Button type='submit' disabled={save.isPending || !form.formState.isDirty}>
            {save.isPending && <Loader2 className='animate-spin' />}
            保存公告
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={save.isPending || !form.formState.isDirty}
            onClick={() => form.reset({ site_notice: saved?.value ?? '' })}
          >
            撤销修改
          </Button>
        </div>
      </form>
    </Form>
  )
}
