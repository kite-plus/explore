import { useState } from 'react'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { formatDateTime } from '@/admin/lib/format'
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
import { Label } from '@/admin/components/ui/label'
import { Textarea } from '@/admin/components/ui/textarea'
import type { Takedown } from '@/lib/admin-types'
import { takedownStatuses } from '../data/data'
import { targetName } from './takedowns-provider'

type Decision = 'approved' | 'rejected'

type TakedownReviewDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  item: Takedown
}

export function TakedownReviewDialog({ open, onOpenChange, item }: TakedownReviewDialogProps) {
  const [note, setNote] = useState('')
  const [decision, setDecision] = useState<Decision | null>(null)
  const review = useAdminMutation((value: Decision) => ({
    path: `/takedowns/${encodeURIComponent(item.id)}/review`,
    method: 'POST',
    body: { decision: value, review_note: note.trim() },
  }))
  const pending = item.status === 'pending'
  const status = takedownStatuses.find((entry) => entry.value === item.status)

  async function decide(value: Decision) {
    setDecision(value)
    try {
      await review.mutateAsync(value)
      toast.success(value === 'approved' ? '已通过并下架' : '申请已驳回', {
        description: targetName(item),
      })
      onOpenChange(false)
    } catch (cause) {
      toastError(cause)
    } finally {
      setDecision(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !review.isPending && onOpenChange(state)}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-start'>
          <DialogTitle>{pending ? '处理下架申请' : '下架申请'}</DialogTitle>
          <DialogDescription>
            {pending
              ? item.target_type === 'blog'
                ? '通过后博客会被暂停，不再抓取和展示。'
                : '通过后这篇文章会被隐藏，重新抓取后仍然隐藏。'
              : `${status?.label ?? ''}，${item.reviewed_by || '—'} 处理于 ${formatDateTime(item.reviewed_at)}`}
          </DialogDescription>
        </DialogHeader>
        <dl className='grid grid-cols-[4.5rem_1fr] gap-x-4 gap-y-2 rounded-md border bg-muted/40 p-3 text-sm'>
          <dt className='text-muted-foreground'>对象</dt>
          <dd className='min-w-0 font-medium break-words'>{targetName(item)}</dd>
          {item.target_type === 'entry' && (
            <>
              <dt className='text-muted-foreground'>博客</dt>
              <dd className='font-mono'>{item.blog_host}</dd>
            </>
          )}
          <dt className='text-muted-foreground'>发起人</dt>
          <dd>{item.requester || '管理员'}</dd>
          <dt className='text-muted-foreground'>提交时间</dt>
          <dd>{formatDateTime(item.created_at)}</dd>
          <dt className='text-muted-foreground'>原因</dt>
          <dd className='whitespace-pre-wrap'>{item.reason}</dd>
          {!pending && item.review_note && (
            <>
              <dt className='text-muted-foreground'>处理备注</dt>
              <dd className='whitespace-pre-wrap'>{item.review_note}</dd>
            </>
          )}
        </dl>
        {pending && (
          <div className='grid gap-2'>
            <Label htmlFor='review-note'>处理备注</Label>
            <Textarea
              id='review-note'
              rows={2}
              maxLength={500}
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder='驳回时必填'
            />
          </div>
        )}
        {pending && (
          <DialogFooter>
            <Button
              variant='outline'
              disabled={review.isPending || !note.trim()}
              onClick={() => void decide('rejected')}
            >
              {decision === 'rejected' && <Loader2 className='animate-spin' />}
              驳回
            </Button>
            <Button variant='destructive' disabled={review.isPending} onClick={() => void decide('approved')}>
              {decision === 'approved' && <Loader2 className='animate-spin' />}
              通过并下架
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
