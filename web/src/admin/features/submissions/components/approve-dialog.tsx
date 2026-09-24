import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { CircleAlert, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { errorMessage, useAdminMutation } from '@/admin/lib/api'
import { optionalUrlField } from '@/admin/lib/validation'
import { Alert, AlertDescription } from '@/admin/components/ui/alert'
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
import { Switch } from '@/admin/components/ui/switch'
import { TagInput } from '@/admin/components/tag-input'
import type { AdminBlog, AdminSubmission, ApprovePayload } from '@/lib/admin-types'

const formSchema = z.object({
  name: z.string().trim().max(200, '名称太长了。'),
  language: z.string().trim().max(35, '语言代码太长了。'),
  feed_url: optionalUrlField,
  extra_domains: z.array(z.string()),
  show_excerpt: z.boolean(),
})
type ApproveForm = z.infer<typeof formSchema>

type ApproveDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  submission: AdminSubmission
}

export function ApproveDialog({ open, onOpenChange, submission }: ApproveDialogProps) {
  const report = submission.check_report
  const approve = useAdminMutation<ApprovePayload, AdminBlog>((payload) => ({
    path: `/submissions/${encodeURIComponent(submission.id)}/approve`,
    method: 'POST',
    body: payload,
  }))
  const form = useForm<ApproveForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { name: '', language: '', feed_url: '', extra_domains: [], show_excerpt: true },
  })
  const rootError = form.formState.errors.root?.message

  const onSubmit = async (values: ApproveForm) => {
    const payload: ApprovePayload = { show_excerpt: values.show_excerpt }
    if (values.name) payload.name = values.name
    if (values.language) payload.language = values.language
    if (values.feed_url) payload.feed_url = values.feed_url
    if (values.extra_domains.length) payload.extra_domains = values.extra_domains
    try {
      const blog = await approve.mutateAsync(payload)
      toast.success(`${blog.host} 已收录`, { description: '博客已进入抓取队列。' })
      onOpenChange(false)
    } catch (cause) {
      const message = errorMessage(cause)
      if (message) form.setError('root', { message })
    }
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !approve.isPending && onOpenChange(state)}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-start'>
          <DialogTitle>通过收录</DialogTitle>
          <DialogDescription>
            通过 <span className='font-mono'>{submission.host}</span>。留空的字段使用检查时检测到的值。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='approve-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <div className='grid gap-4 sm:grid-cols-[1fr_8rem]'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={report?.title || submission.host} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='language'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>语言</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={report?.language || 'zh-CN'} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='feed_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>订阅源地址</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder={submission.feed_url} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='extra_domains'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>额外允许的域名</FormLabel>
                  <FormControl>
                    <TagInput value={field.value} onChange={field.onChange} placeholder='例如 notes.example.com，按 Enter 添加' />
                  </FormControl>
                  <FormDescription>文章链接指向这些域名时，仍算作这个博客的文章。</FormDescription>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='show_excerpt'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-4'>
                  <div className='space-y-0.5'>
                    <FormLabel className='text-base'>展示文章摘要</FormLabel>
                    <FormDescription>关闭后前台只显示标题和链接。</FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
            {rootError && (
              <Alert variant='destructive'>
                <CircleAlert />
                <AlertDescription>{rootError}</AlertDescription>
              </Alert>
            )}
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='approve-form' disabled={approve.isPending}>
            {approve.isPending && <Loader2 className='animate-spin' />}
            确认通过
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
