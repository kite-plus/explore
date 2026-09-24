import { useQuery } from '@tanstack/react-query'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { toastError, useAdminMutation } from '@/admin/lib/api'
import { optionalUrlField } from '@/admin/lib/validation'
import { Button } from '@/admin/components/ui/button'
import { Checkbox } from '@/admin/components/ui/checkbox'
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
import { RadioGroup, RadioGroupItem } from '@/admin/components/ui/radio-group'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/admin/components/ui/sheet'
import { Switch } from '@/admin/components/ui/switch'
import { Textarea } from '@/admin/components/ui/textarea'
import { TagInput } from '@/admin/components/tag-input'
import type { AdminBlog, UpdateBlogPayload } from '@/lib/admin-types'
import type { Tag } from '@/lib/types'

const formSchema = z.object({
  name: z.string().trim().max(200, '名称太长了。'),
  language: z.string().trim().max(35, '语言代码太长了。'),
  feed_url: optionalUrlField,
  extra_domains: z.array(z.string()),
  default_tags: z.array(z.string()).max(3, '最多选择 3 个标签。'),
  show_excerpt: z.boolean(),
  status: z.enum(['active', 'paused']),
  status_note: z.string().trim().max(500, '暂停原因最多 500 字。'),
})
type BlogForm = z.infer<typeof formSchema>

type BlogsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  blog: AdminBlog
}

export function BlogsMutateDrawer({ open, onOpenChange, blog }: BlogsMutateDrawerProps) {
  const tags = useQuery({
    queryKey: ['tags'],
    queryFn: async () => {
      const response = await fetch('/api/v1/tags')
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      return ((await response.json()) as { data: Tag[] }).data
    },
    staleTime: Infinity,
  })
  const update = useAdminMutation((payload: UpdateBlogPayload) => ({
    path: `/blogs/${encodeURIComponent(blog.host)}`,
    method: 'PATCH',
    body: payload,
  }))

  const form = useForm<BlogForm>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: blog.name,
      language: blog.language,
      feed_url: blog.feed_url,
      extra_domains: blog.extra_domains ?? [],
      default_tags: blog.default_tags ?? [],
      show_excerpt: blog.show_excerpt,
      status: blog.status,
      status_note: blog.status_note ?? '',
    },
  })
  const status = form.watch('status')

  const onSubmit = async (values: BlogForm) => {
    try {
      await update.mutateAsync({
        name: values.name || undefined,
        language: values.language || undefined,
        feed_url: values.feed_url || undefined,
        extra_domains: values.extra_domains,
        default_tags: tags.isError ? undefined : values.default_tags,
        show_excerpt: values.show_excerpt,
        status: values.status,
        status_note: values.status_note || undefined,
      })
      toast.success(`${blog.host} 的资料已保存`)
      onOpenChange(false)
    } catch (cause) {
      toastError(cause, '保存失败，请重试。')
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(value) => {
        onOpenChange(value)
        form.reset()
      }}
    >
      <SheetContent className='flex flex-col sm:max-w-lg'>
        <SheetHeader className='text-start'>
          <SheetTitle>编辑博客</SheetTitle>
          <SheetDescription>
            修改 {blog.host} 的资料和运行状态，完成后点击保存。
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='blogs-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex-1 space-y-6 overflow-y-auto px-4'
          >
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>名称</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='博客名称' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid gap-6 sm:grid-cols-[8rem_1fr]'>
              <FormField
                control={form.control}
                name='language'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>语言</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder='zh-CN' />
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
                      <Input {...field} placeholder='https://example.com/feed.xml' />
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
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='default_tags'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>默认标签</FormLabel>
                  <FormDescription>最多 3 个，作为模型给文章打标签时的参考。</FormDescription>
                  {tags.isError ? (
                    <p className='text-sm text-destructive'>标签列表加载失败，暂时不能修改默认标签。</p>
                  ) : (
                    <div className='grid grid-cols-2 gap-2 pt-1 sm:grid-cols-3'>
                      {(tags.data ?? []).map((tag) => (
                        <label key={tag.slug} className='flex items-center gap-2 text-sm'>
                          <Checkbox
                            checked={field.value.includes(tag.slug)}
                            onCheckedChange={(checked) =>
                              field.onChange(
                                checked
                                  ? [...field.value, tag.slug]
                                  : field.value.filter((slug) => slug !== tag.slug)
                              )
                            }
                          />
                          {tag.name.zh}
                        </label>
                      ))}
                    </div>
                  )}
                  <FormMessage />
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
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>运行状态</FormLabel>
                  <FormControl>
                    <RadioGroup
                      onValueChange={field.onChange}
                      value={field.value}
                      className='flex gap-6'
                    >
                      <FormItem className='flex items-center'>
                        <FormControl>
                          <RadioGroupItem value='active' />
                        </FormControl>
                        <FormLabel className='font-normal'>运行中</FormLabel>
                      </FormItem>
                      <FormItem className='flex items-center'>
                        <FormControl>
                          <RadioGroupItem value='paused' />
                        </FormControl>
                        <FormLabel className='font-normal'>已暂停</FormLabel>
                      </FormItem>
                    </RadioGroup>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {status === 'paused' && (
              <FormField
                control={form.control}
                name='status_note'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>暂停原因</FormLabel>
                    <FormControl>
                      <Textarea {...field} rows={3} placeholder='仅后台可见' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
          </form>
        </Form>
        <SheetFooter className='gap-2'>
          <SheetClose asChild>
            <Button variant='outline'>关闭</Button>
          </SheetClose>
          <Button form='blogs-form' type='submit' disabled={update.isPending}>
            {update.isPending && <Loader2 className='animate-spin' />}
            保存修改
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
