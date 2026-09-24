import { Check, ExternalLink, X } from 'lucide-react'
import { formatDateTime } from '@/admin/lib/format'
import { Badge } from '@/admin/components/ui/badge'
import { Button } from '@/admin/components/ui/button'
import { Separator } from '@/admin/components/ui/separator'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/admin/components/ui/sheet'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { CheckProblems } from '@/admin/components/check-problems'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminSubmission } from '@/lib/admin-types'
import { submissionStatuses } from '../data/data'

type SubmissionDetailDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onApprove: () => void
  onReject: () => void
  submission: AdminSubmission
}

export function SubmissionDetailDrawer({
  open,
  onOpenChange,
  onApprove,
  onReject,
  submission,
}: SubmissionDetailDrawerProps) {
  const report = submission.check_report
  const status = submissionStatuses.find((item) => item.value === submission.status)
  const reviewer =
    submission.reviewed_by === 'system' ? '已收录，系统自动归档' : submission.reviewed_by

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col sm:max-w-lg'>
        <SheetHeader className='text-start'>
          <div className='flex items-center gap-3'>
            <BlogAvatar host={submission.host} name={report?.title || submission.host} large />
            <div className='min-w-0'>
              <SheetTitle className='truncate'>{report?.title || submission.host}</SheetTitle>
              <SheetDescription className='truncate font-mono'>{submission.host}</SheetDescription>
            </div>
          </div>
          <div className='flex flex-wrap gap-2 pt-2'>
            {status && (
              <Badge variant='outline' className={status.className}>
                <status.icon />
                {status.label}
              </Badge>
            )}
            {report && (
              <Badge variant={report.passed ? 'secondary' : 'destructive'}>
                {report.passed ? '检查通过' : '检查未通过'}
              </Badge>
            )}
          </div>
        </SheetHeader>
        <div className='flex-1 space-y-6 overflow-y-auto px-4'>
          {report?.description && <p className='text-sm text-muted-foreground'>{report.description}</p>}
          <dl className='grid grid-cols-[6rem_1fr] gap-x-4 gap-y-2.5 text-sm'>
            <dt className='text-muted-foreground'>网站</dt>
            <dd className='break-all'>
              <a href={submission.site_url} target='_blank' rel='noopener' className='hover:underline'>
                {submission.site_url}
              </a>
            </dd>
            <dt className='text-muted-foreground'>订阅源</dt>
            <dd className='break-all'>
              <a href={submission.feed_url} target='_blank' rel='noopener' className='hover:underline'>
                {submission.feed_url}
              </a>
              {report?.discovered_by && (
                <span className='text-muted-foreground'>（{report.discovered_by}）</span>
              )}
            </dd>
            {report && (report.generator || report.language) && (
              <>
                <dt className='text-muted-foreground'>程序 / 语言</dt>
                <dd>
                  {report.generator || '—'} / {report.language || '—'}
                </dd>
              </>
            )}
            {report?.items && (
              <>
                <dt className='text-muted-foreground'>文章</dt>
                <dd>
                  {report.items.valid} 篇有效，共 {report.items.total} 篇
                  {report.items.latest_published_at && (
                    <span className='text-muted-foreground'>
                      ，最近发布于 <TimeAgo iso={report.items.latest_published_at} />
                    </span>
                  )}
                </dd>
              </>
            )}
            <dt className='text-muted-foreground'>提交时间</dt>
            <dd>{formatDateTime(submission.created_at)}</dd>
            {submission.note && (
              <>
                <dt className='text-muted-foreground'>提交备注</dt>
                <dd className='whitespace-pre-wrap'>{submission.note}</dd>
              </>
            )}
            {submission.status !== 'pending' && (
              <>
                <dt className='text-muted-foreground'>处理</dt>
                <dd>
                  {reviewer || '—'}，{formatDateTime(submission.reviewed_at)}
                </dd>
              </>
            )}
            {submission.status === 'rejected' && submission.review_note && (
              <>
                <dt className='text-muted-foreground'>驳回原因</dt>
                <dd className='text-destructive'>{submission.review_note}</dd>
              </>
            )}
          </dl>
          <Separator />
          <section className='space-y-3'>
            <h3 className='text-sm font-medium'>检查结果</h3>
            <CheckProblems problems={report?.problems} />
          </section>
        </div>
        <SheetFooter>
          <Button variant='outline' asChild>
            <a href={submission.site_url} target='_blank' rel='noopener'>
              <ExternalLink />
              打开网站
            </a>
          </Button>
          {submission.status === 'pending' && (
            <>
              <Button variant='outline' onClick={onReject}>
                <X />
                驳回
              </Button>
              <Button onClick={onApprove}>
                <Check />
                通过
              </Button>
            </>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
