import { useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { toast } from 'sonner'
import { toastError, useAdminMutation } from '@/admin/lib/api'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/admin/components/ui/radio-group'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import type { AdminBlog } from '@/lib/admin-types'

type Exclude = '' | 'opt_out' | 'blocked'

const OPTIONS: { value: Exclude; label: string; description: string }[] = [
  { value: '', label: '仅移除', description: '以后仍可重新提交收录。' },
  { value: 'opt_out', label: '移除并记为申请退出', description: '博主要求退出时使用，域名进入排除名单。' },
  { value: 'blocked', label: '移除并永久封禁', description: '违规时使用，域名进入排除名单，不能再提交。' },
]

type BlogsDeleteDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  blog: AdminBlog
}

export function BlogsDeleteDialog({ open, onOpenChange, blog }: BlogsDeleteDialogProps) {
  const [exclude, setExclude] = useState<Exclude>('')
  const [note, setNote] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const remove = useAdminMutation(() => {
    const query = exclude ? `?${new URLSearchParams({ exclude, note: note.trim() })}` : ''
    return { path: `/blogs/${encodeURIComponent(blog.host)}${query}`, method: 'DELETE' }
  })

  async function handleDelete() {
    try {
      await remove.mutateAsync(undefined)
      toast.success(`${blog.host} 已移除`)
      onOpenChange(false)
    } catch (cause) {
      toastError(cause, '移除失败，请重试。')
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      handleConfirm={() => void handleDelete()}
      disabled={confirmation.trim() !== blog.host}
      isLoading={remove.isPending}
      title={
        <span className='text-destructive'>
          <TriangleAlert className='me-1 inline-block stroke-destructive' size={18} /> 移除博客
        </span>
      }
      desc={
        <div className='space-y-4'>
          <p className='mb-2'>
            确定要移除 <span className='font-bold'>{blog.host}</span> 吗？
            <br />
            博客和已缓存的文章都会被删除。
          </p>
          <RadioGroup value={exclude} onValueChange={(value) => setExclude(value as Exclude)} className='gap-3'>
            {OPTIONS.map((option) => (
              <label key={option.value || 'none'} className='flex items-start gap-3 text-start'>
                <RadioGroupItem value={option.value} className='mt-0.5' />
                <span>
                  <span className='block font-medium text-foreground'>{option.label}</span>
                  <span className='block text-sm'>{option.description}</span>
                </span>
              </label>
            ))}
          </RadioGroup>
          {exclude && (
            <Label className='my-2 flex-col items-start gap-2'>
              说明（可选，仅后台可见）
              <Input value={note} onChange={(event) => setNote(event.target.value)} placeholder='补充说明' />
            </Label>
          )}
          <Label className='my-2 flex-col items-start gap-2'>
            输入域名以确认：
            <Input
              value={confirmation}
              onChange={(event) => setConfirmation(event.target.value)}
              placeholder={blog.host}
            />
          </Label>
          <Alert variant='destructive'>
            <AlertTitle>注意</AlertTitle>
            <AlertDescription>这项操作不能撤销。</AlertDescription>
          </Alert>
        </div>
      }
      confirmText='移除'
      destructive
    />
  )
}
