import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { CircleAlert, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { errorMessage, useAdminMutation } from '@/admin/lib/api'
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
import { RadioGroup, RadioGroupItem } from '@/admin/components/ui/radio-group'
import { Textarea } from '@/admin/components/ui/textarea'

const formSchema = z
  .object({
    target_type: z.enum(['blog', 'entry']),
    blog_host: z.string().trim().toLowerCase().min(1, '请填写博客域名。'),
    entry_id: z.string().trim(),
    reason: z.string().trim().min(5, '原因至少 5 个字。').max(1000, '原因最多 1000 字。'),
  })
  .refine((values) => values.target_type === 'blog' || /^[1-9]\d*$/.test(values.entry_id), {
    message: '请填写文章编号。',
    path: ['entry_id'],
  })
type TakedownForm = z.infer<typeof formSchema>

type Payload = { target_type: 'blog' | 'entry'; blog_host: string; entry_id: number; reason: string }

export function TakedownCreateDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const create = useAdminMutation((payload: Payload) => ({ path: '/takedowns', method: 'POST', body: payload }))
  const form = useForm<TakedownForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { target_type: 'blog', blog_host: '', entry_id: '', reason: '' },
  })
  const targetType = form.watch('target_type')
  const rootError = form.formState.errors.root?.message

  const onSubmit = async (values: TakedownForm) => {
    try {
      await create.mutateAsync({
        target_type: values.target_type,
        blog_host: values.blog_host,
        entry_id: values.target_type === 'entry' ? Number(values.entry_id) : 0,
        reason: values.reason,
      })
      toast.success('下架申请已创建', { description: '在待处理列表里审批后才会生效。' })
      form.reset()
      onOpenChange(false)
    } catch (cause) {
      const message = errorMessage(cause)
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
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-start'>
          <DialogTitle>新建下架申请</DialogTitle>
          <DialogDescription>申请进入待处理列表，审批通过后才会下架。</DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form id='takedown-form' onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 px-0.5'>
            <FormField
              control={form.control}
              name='target_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>对象</FormLabel>
                  <FormControl>
                    <RadioGroup onValueChange={field.onChange} value={field.value} className='flex gap-6'>
                      <FormItem className='flex items-center'>
                        <FormControl>
                          <RadioGroupItem value='blog' />
                        </FormControl>
                        <FormLabel className='font-normal'>整个博客</FormLabel>
                      </FormItem>
                      <FormItem className='flex items-center'>
                        <FormControl>
                          <RadioGroupItem value='entry' />
                        </FormControl>
                        <FormLabel className='font-normal'>单篇文章</FormLabel>
                      </FormItem>
                    </RadioGroup>
                  </FormControl>
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='blog_host'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>博客域名</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder='example.com' className='font-mono' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            {targetType === 'entry' && (
              <FormField
                control={form.control}
                name='entry_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>文章编号</FormLabel>
                    <FormControl>
                      <Input {...field} inputMode='numeric' placeholder='例如 1024' />
                    </FormControl>
                    <FormDescription>文章管理里每篇文章标题下 # 后面的数字。</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}
            <FormField
              control={form.control}
              name='reason'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>申请原因</FormLabel>
                  <FormControl>
                    <Textarea {...field} rows={3} placeholder='至少 5 个字' />
                  </FormControl>
                  <FormMessage />
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
          <Button type='submit' form='takedown-form' disabled={create.isPending}>
            {create.isPending && <Loader2 className='animate-spin' />}
            创建申请
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
