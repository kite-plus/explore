import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, CircleX, Loader2, Plus, Search } from 'lucide-react'
import { errorMessage, useAdminRequest } from '@/admin/lib/api'
import { optionalUrlField, urlField } from '@/admin/lib/validation'
import { Link } from '@/admin/router'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { Badge } from '@/admin/components/ui/badge'
import { Button } from '@/admin/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/admin/components/ui/card'
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
import { Separator } from '@/admin/components/ui/separator'
import { CheckProblems, problemCounts } from '@/admin/components/check-problems'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { TimeAgo } from '@/admin/components/time-ago'
import { BlogsActionDialog } from '@/admin/features/blogs/components/blogs-action-dialog'
import type { AdminBlog } from '@/lib/admin-types'
import type { CheckReport } from '@/lib/types'

const formSchema = z.object({
  url: urlField,
  feed_url: optionalUrlField,
})
type CheckForm = z.infer<typeof formSchema>

export function Tools() {
  const request = useAdminRequest()
  const [adding, setAdding] = useState(false)
  const [created, setCreated] = useState<AdminBlog | null>(null)
  const form = useForm<CheckForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { url: '', feed_url: '' },
  })
  const check = useMutation({
    mutationFn: (values: CheckForm) =>
      request<CheckReport>('/check', {
        method: 'POST',
        body: JSON.stringify(values.feed_url ? values : { url: values.url }),
      }),
    onMutate: () => setCreated(null),
  })
  const report = check.data
  const failure = check.error ? errorMessage(check.error, '检查失败，请确认网址后重试。') : null
  const counts = problemCounts(report?.problems)

  return (
    <>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='诊断工具' description='检查任意博客的网站和订阅源，结果和投稿检查一致。' />

        <Card>
          <CardHeader>
            <CardTitle>检查博客</CardTitle>
            <CardDescription>通常需要 5 到 30 秒，取决于对方网站的响应速度。</CardDescription>
          </CardHeader>
          <CardContent>
            <Form {...form}>
              <form onSubmit={form.handleSubmit((values) => check.mutate(values))} className='space-y-4'>
                <div className='grid gap-4 md:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>博客网址</FormLabel>
                        <FormControl>
                          <Input {...field} placeholder='https://example.com' />
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
                        <FormDescription>发现的订阅源不对时再填写。</FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
                <Button type='submit' disabled={check.isPending}>
                  {check.isPending ? <Loader2 className='animate-spin' /> : <Search />}
                  {check.isPending ? '正在检查…' : '开始检查'}
                </Button>
              </form>
            </Form>
          </CardContent>
        </Card>

        {failure && (
          <Alert variant='destructive'>
            <CircleAlert />
            <AlertTitle>检查失败</AlertTitle>
            <AlertDescription>{failure}</AlertDescription>
          </Alert>
        )}

        {report && (
          <Card>
            <CardHeader className='flex flex-row flex-wrap items-start justify-between gap-4 space-y-0'>
              <div className='space-y-1.5'>
                <CardTitle className='flex items-center gap-2'>
                  {report.passed ? (
                    <CircleCheck className='size-5 text-muted-foreground' />
                  ) : (
                    <CircleX className='size-5 text-destructive' />
                  )}
                  {report.passed ? '检查通过，可以收录' : '检查未通过'}
                </CardTitle>
                <CardDescription>{report.title || report.input_url}</CardDescription>
              </div>
              <div className='flex items-center gap-2'>
                {counts.errors > 0 && <Badge variant='destructive'>{counts.errors} 个错误</Badge>}
                {counts.warnings > 0 && <Badge variant='secondary'>{counts.warnings} 个警告</Badge>}
                {report.passed &&
                  (created ? (
                    <Button variant='outline' asChild>
                      <Link to={`/admin/blogs?filter=${encodeURIComponent(created.host)}`}>查看博客</Link>
                    </Button>
                  ) : (
                    <Button onClick={() => setAdding(true)}>
                      <Plus />
                      添加这个博客
                    </Button>
                  ))}
              </div>
            </CardHeader>
            <CardContent className='space-y-6'>
              <dl className='grid gap-x-6 gap-y-2.5 text-sm sm:grid-cols-[6rem_1fr]'>
                {report.description && (
                  <>
                    <dt className='text-muted-foreground'>简介</dt>
                    <dd>{report.description}</dd>
                  </>
                )}
                {report.feed_url && (
                  <>
                    <dt className='text-muted-foreground'>订阅源</dt>
                    <dd className='break-all'>
                      {report.feed_url}
                      {report.discovered_by && (
                        <span className='text-muted-foreground'>（{report.discovered_by}）</span>
                      )}
                    </dd>
                  </>
                )}
                {(report.format || report.generator || report.language) && (
                  <>
                    <dt className='text-muted-foreground'>格式</dt>
                    <dd>{[report.format, report.generator, report.language].filter(Boolean).join(' · ')}</dd>
                  </>
                )}
                {report.items && (
                  <>
                    <dt className='text-muted-foreground'>文章</dt>
                    <dd>
                      {report.items.valid} 篇有效，共 {report.items.total} 篇，{report.items.trusted_dates} 篇日期可信
                    </dd>
                  </>
                )}
                {report.latest_entry_title && (
                  <>
                    <dt className='text-muted-foreground'>最新文章</dt>
                    <dd>
                      {report.latest_entry_title}
                      {report.items?.latest_published_at && (
                        <span className='text-muted-foreground'>
                          ，<TimeAgo iso={report.items.latest_published_at} />
                        </span>
                      )}
                    </dd>
                  </>
                )}
              </dl>
              <Separator />
              <section className='space-y-3'>
                <h3 className='text-sm font-medium'>检查结果</h3>
                <CheckProblems problems={report.problems} />
              </section>
            </CardContent>
          </Card>
        )}
      </Main>

      <BlogsActionDialog
        key={report?.input_url ?? 'none'}
        open={adding}
        onOpenChange={setAdding}
        initialURL={form.getValues('url')}
        onCreated={setCreated}
      />
    </>
  )
}
