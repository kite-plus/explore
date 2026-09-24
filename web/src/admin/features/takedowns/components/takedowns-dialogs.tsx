import { TakedownCreateDialog } from './takedown-create-dialog'
import { TakedownReviewDialog } from './takedown-review-dialog'
import { useTakedowns } from './takedowns-provider'

export function TakedownsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useTakedowns()
  return (
    <>
      <TakedownCreateDialog key='takedown-create' open={open === 'create'} onOpenChange={() => setOpen('create')} />
      {currentRow && (
        <TakedownReviewDialog
          key={`takedown-review-${currentRow.id}`}
          open={open === 'review'}
          onOpenChange={() => {
            setOpen('review')
            setTimeout(() => setCurrentRow(null), 500)
          }}
          item={currentRow}
        />
      )}
    </>
  )
}
