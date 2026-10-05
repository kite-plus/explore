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
import { Input } from '@/admin/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/admin/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/admin/components/ui/select'
import { Switch } from '@/admin/components/ui/switch'
import { Textarea } from '@/admin/components/ui/textarea'
import type { AdminNotice, NoticePayload } from '@/lib/admin-types'

// The same limits the API checks; docs/design/notices.md section 1.
const formSchema = z
  .object({
    kind: z.enum(['notice', 'ad']),
    title: z.string().trim().min(1, '请填写标题。').max(120, '标题最多 120 字。'),
    summary: z.string().trim().max(280, '介绍最多 280 字。'),
    body: z.string().trim().max(20000, '正文最多 2 万字。'),
    url: z
      .string()
      .trim()
      .max(2000, '链接太长了。')
      .refine((v) => v === '' || /^https?:\/\/[^\s/]+/i.test(v), '链接要以 http:// 或 https:// 开头。'),
    source_name: z.string().trim().min(1, '请填写显示的名字。').max(60, '名字最多 60 字。'),
    position: z.coerce.number<number>().int('请填整数。').min(0, '最小是 0。').max(50, '最大是 50。'),
    audience: z.enum(['all', 'zh', 'en']),
    starts_at: z.string(),
    ends_at: z.string(),
    enabled: z.boolean(),
  })
  .refine((v) => v.url !== '' || v.body !== '', { path: ['body'], message: '正文和链接至少填一个。' })
  .refine((v) => !v.starts_at || !v.ends_at || new Date(v.ends_at) > new Date(v.starts_at), {
    path: ['ends_at'],
    message: '结束时间要晚于开始时间。',
  })
type NoticeForm = z.infer<typeof formSchema>

// datetime-local inputs speak the browser's local time, without a zone.
function toLocalInput(iso: string | null) {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const fromLocalInput = (value: string) => (value ? new Date(value).toISOString() : null)

function defaults(notice: AdminNotice | null): NoticeForm {
  return {
    kind: notice?.kind ?? 'notice',
    title: notice?.title ?? '',
    summary: notice?.summary ?? '',
    body: notice?.body ?? '',
    url: notice?.url ?? '',
    source_name: notice?.source_name ?? 'Kite Plus',
    position: notice?.position ?? 0,
    audience: notice?.audience || 'all',
    starts_at: toLocalInput(notice?.starts_at ?? null),
    ends_at: toLocalInput(notice?.ends_at ?? null),
    enabled: notice?.enabled ?? false,
  }
}

type NoticeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The notice to edit, or null for a new one. */
  notice: AdminNotice | null
}

export function NoticeDialog({ open, onOpenChange, notice }: NoticeDialogProps) {
  const form = useForm<NoticeForm>({ resolver: zodResolver(formSchema), values: defaults(notice) })
  const save = useAdminMutation((payload: NoticePayload) =>
    notice
      ? { path: `/notices/${notice.id}`, method: 'PATCH', body: payload }
      : { path: '/notices', method: 'POST', body: payload }
  )
  const kind = form.watch('kind')
  const busy = save.isPending

  async function onSubmit(values: NoticeForm) {
    const payload: NoticePayload = {
      ...values,
      audience: values.audience === 'all' ? '' : values.audience,
      starts_at: fromLocalInput(values.starts_at),
      ends_at: fromLocalInput(values.ends_at),
    }
    try {
      await save.mutateAsync(payload)
      toast.success(notice ? '已保存' : values.enabled ? '已发布' : '已存为草稿')
      onOpenChange(false)
    } catch (cause) {
      toastError(cause, '保存失败，请重试。')
    }
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !busy && onOpenChange(state)}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-xl'>
        <DialogHeader className='text-start'>
          <DialogTitle>{notice ? '编辑' : '新建公告或广告'}</DialogTitle>
          <DialogDescription>显示在最新流第一页，和文章同一种行；有链接的直接打开链接，没有的打开 Explore 上的条目页。</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='notice-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-5 px-0.5'>
            <FormField
              control={form.control}
              name='kind'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>类型</FormLabel>
                  <FormControl>
                    <RadioGroup value={field.value} onValueChange={field.onChange} className='flex gap-6'>
                      <label className='flex items-center gap-2 text-sm'>
                        <RadioGroupItem value='notice' />
                        公告
                      </label>
                      <label className='flex items-center gap-2 text-sm'>
                        <RadioGroupItem value='ad' />
                        广告
                      </label>
                    </RadioGroup>
                  </FormControl>
                  {kind === 'ad' && <FormDescription>广告在每个出现的地方都带「广告」标记，不能去掉。</FormDescription>}
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='title'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>标题</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='summary'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>介绍</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={2} placeholder='列表里标题下面的一两句，可以不填。' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>链接</FormLabel>
                  <FormControl>
                    <Input {...field} type='url' placeholder='https://' autoComplete='off' />
                  </FormControl>
                  <FormDescription>填了就直接打开这个地址，广告的地址会加上 utm_source。</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='body'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>正文</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={6} placeholder='条目页上的正文，纯文本，空一行分段。' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <div className='grid gap-5 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='source_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>显示的名字</FormLabel>
                    <FormControl>
                      <Input {...field} autoComplete='off' />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='position'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>位置</FormLabel>
                    <FormControl>
                      <Input {...field} type='number' min={0} max={50} />
                    </FormControl>
                    <FormDescription>0 置顶；N 插在第 N 篇文章之后。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='starts_at'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>开始</FormLabel>
                    <FormControl>
                      <Input {...field} type='datetime-local' />
                    </FormControl>
                    <FormDescription>不填就是立即。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='ends_at'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>结束</FormLabel>
                    <FormControl>
                      <Input {...field} type='datetime-local' />
                    </FormControl>
                    <FormDescription>不填就一直显示。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='audience'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>给谁看</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger className='w-full sm:w-60'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='all'>所有人</SelectItem>
                      <SelectItem value='zh'>中文界面</SelectItem>
                      <SelectItem value='en'>英文界面</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>发布</FormLabel>
                    <FormDescription>关掉就是草稿，读者看不到。</FormDescription>
                  </div>
                  <FormControl>
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                  </FormControl>
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button type='submit' form='notice-form' disabled={busy}>
            {busy && <Loader2 className='animate-spin' />}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
