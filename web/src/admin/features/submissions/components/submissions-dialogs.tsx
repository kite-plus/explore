import { ApproveDialog } from './approve-dialog'
import { RejectDialog } from './reject-dialog'
import { SubmissionDetailDrawer } from './submission-detail-drawer'
import { useSubmissions } from './submissions-provider'

export function SubmissionsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useSubmissions()
  if (!currentRow) return null
  const close = (dialog: 'detail' | 'approve' | 'reject') => () => {
    setOpen(dialog)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <SubmissionDetailDrawer
        key={`submission-detail-${currentRow.id}`}
        open={open === 'detail'}
        onOpenChange={close('detail')}
        onApprove={() => setOpen('approve')}
        onReject={() => setOpen('reject')}
        submission={currentRow}
      />
      <ApproveDialog
        key={`submission-approve-${currentRow.id}`}
        open={open === 'approve'}
        onOpenChange={close('approve')}
        submission={currentRow}
      />
      <RejectDialog
        key={`submission-reject-${currentRow.id}`}
        open={open === 'reject'}
        onOpenChange={close('reject')}
        submission={currentRow}
      />
    </>
  )
}
