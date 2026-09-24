import { useEffect } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { formatDateTime } from '@/admin/lib/format'
import { toastError, useAdminRequest } from '@/admin/lib/api'
import { useQueryClient } from '@tanstack/react-query'
import { Button } from '@/admin/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/admin/components/ui/form'
import { Skeleton } from '@/admin/components/ui/skeleton'
import { Switch } from '@/admin/components/ui/switch'
import { QueryError } from '@/admin/components/query-error'
import { useSettings } from '../settings-api'

const SWITCHES = [
  { key: 'registration_enabled', label: '开放注册', description: '关闭后不能创建新账号，已有账号仍可登录。' },
  { key: 'submissions_enabled', label: '开放投稿', description: '关闭后前台不再接受新的博客收录申请。' },
  { key: 'crawler_paused', label: '暂停抓取任务', description: '开启后抓取进程不再领取新任务，队列保留。' },
] as const

const formSchema = z.object({
  registration_enabled: z.boolean(),
  submissions_enabled: z.boolean(),
  crawler_paused: z.boolean(),
})
type GeneralFormValues = z.infer<typeof formSchema>

export function GeneralForm() {
  const { byKey, isPending, error, refetch } = useSettings()
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  const form = useForm<GeneralFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { registration_enabled: true, submissions_enabled: true, crawler_paused: false },
  })

  const loaded = !isPending && !error
  useEffect(() => {
    if (!loaded) return
    form.reset({
      registration_enabled: byKey.get('registration_enabled')?.value === 'true',
      submissions_enabled: byKey.get('submissions_enabled')?.value === 'true',
      crawler_paused: byKey.get('crawler_paused')?.value === 'true',
    })
    // Reset only when the stored values change, not on every render.
  }, [loaded, byKey.get('registration_enabled')?.value, byKey.get('submissions_enabled')?.value, byKey.get('crawler_paused')?.value])

  async function onSubmit(values: GeneralFormValues) {
    const changed = SWITCHES.filter(({ key }) => form.formState.dirtyFields[key])
    if (changed.length === 0) return
    try {
      for (const { key } of changed) {
        await request(`/settings/${key}`, {
          method: 'PATCH',
          body: JSON.stringify({ value: String(values[key]) }),
        })
      }
      toast.success('设置已保存')
    } catch (cause) {
      toastError(cause, '保存失败，请重试。')
    } finally {
      await queryClient.invalidateQueries({ queryKey: ['admin'] })
    }
  }

  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />
  if (isPending)
    return (
      <div className='space-y-4'>
        {SWITCHES.map(({ key }) => (
          <Skeleton key={key} className='h-20 w-full' />
        ))}
      </div>
    )

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <div className='space-y-4'>
          {SWITCHES.map(({ key, label, description }) => {
            const setting = byKey.get(key)
            return (
              <FormField
                key={key}
                control={form.control}
                name={key}
                render={({ field }) => (
                  <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                    <div className='space-y-0.5'>
                      <FormLabel className='text-base'>{label}</FormLabel>
                      <FormDescription>
                        {description}
                        {setting && (
                          <span className='mt-1 block text-xs'>
                            {setting.updated_by} 更新于 {formatDateTime(setting.updated_at)}
                          </span>
                        )}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
            )
          })}
        </div>
        <Button type='submit' disabled={form.formState.isSubmitting || !form.formState.isDirty}>
          {form.formState.isSubmitting && <Loader2 className='animate-spin' />}
          保存设置
        </Button>
      </form>
    </Form>
  )
}
