import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { CircleAlert, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { errorMessage, useAdminMutation } from '@/admin/lib/api'
import { optionalUrlField, urlField } from '@/admin/lib/validation'
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
import type { AdminBlog, CreateBlogPayload } from '@/lib/admin-types'

const formSchema = z.object({
  site_url: urlField,
  feed_url: optionalUrlField,
  name: z.string().trim().max(200, '名称太长了。'),
  language: z.string().trim().max(35, '语言代码太长了。'),
  extra_domains: z.array(z.string()),
  show_excerpt: z.boolean(),
})
type BlogCreateForm = z.infer<typeof formSchema>

type BlogsActionDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialURL?: string
  onCreated?: (blog: AdminBlog) => void
}

export function BlogsActionDialog({ open, onOpenChange, initialURL = '', onCreated }: BlogsActionDialogProps) {
  const create = useAdminMutation<CreateBlogPayload, AdminBlog>((payload) => ({
    path: '/blogs',
    method: 'POST',
    body: payload,
  }))
  const form = useForm<BlogCreateForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      site_url: initialURL,
      feed_url: '',
      name: '',
      language: '',
      extra_domains: [],
      show_excerpt: true,
    },
  })
  const rootError = form.formState.errors.root?.message

  const onSubmit = async (values: BlogCreateForm) => {
    try {
      const blog = await create.mutateAsync({
        site_url: values.site_url,
        ...(values.feed_url && { feed_url: values.feed_url }),
        ...(values.name && { name: values.name }),
        ...(values.language && { language: values.language }),
        extra_domains: values.extra_domains,
        show_excerpt: values.show_excerpt,
      })
      toast.success(`${blog.host} 已收录`, { description: '抓取进程会在下一轮领取它。' })
      form.reset()
      onOpenChange(false)
      onCreated?.(blog)
    } catch (cause) {
      const message = errorMessage(cause, '添加失败，请重试。')
      if (message) form.setError('root', { message })
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(state) => {
        if (create.isPending) return
        form.reset()
        onOpenChange(state)
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-start'>
          <DialogTitle>添加博客</DialogTitle>
          <DialogDescription>
            保存前会检查网站和订阅源，通过后立即收录，不经过审核。
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='blog-create-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='site_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>博客网址</FormLabel>
                  <FormControl>
                    <Input {...field} autoFocus placeholder='https://example.com' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='feed_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>订阅源地址</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='留空则自动发现' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid gap-4 sm:grid-cols-[1fr_8rem]'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>名称</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='留空则使用检测到的名称' />
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
                      <Input {...field} placeholder='自动检测' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='extra_domains'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>额外允许的域名</FormLabel>
                  <FormControl>
                    <TagInput
                      value={field.value}
                      onChange={field.onChange}
                      placeholder='例如 notes.example.com，按 Enter 添加'
                    />
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
          <Button type='submit' form='blog-create-form' disabled={create.isPending}>
            {create.isPending && <Loader2 className='animate-spin' />}
            {create.isPending ? '正在检查…' : '检查并添加'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
