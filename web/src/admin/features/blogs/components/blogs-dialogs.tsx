import { BlogDetailDrawer } from './blog-detail-drawer'
import { BlogsActionDialog } from './blogs-action-dialog'
import { BlogsDeleteDialog } from './blogs-delete-dialog'
import { BlogsMutateDrawer } from './blogs-mutate-drawer'
import { useBlogs } from './blogs-provider'

export function BlogsDialogs() {
  const { open, setOpen, currentRow, setCurrentRow } = useBlogs()
  const close = (dialog: 'detail' | 'edit' | 'delete') => () => {
    setOpen(dialog)
    setTimeout(() => setCurrentRow(null), 500)
  }

  return (
    <>
      <BlogsActionDialog key='blog-create' open={open === 'create'} onOpenChange={() => setOpen('create')} />

      {currentRow && (
        <>
          <BlogDetailDrawer
            key={`blog-detail-${currentRow.host}`}
            open={open === 'detail'}
            onOpenChange={close('detail')}
            onEdit={() => setOpen('edit')}
            blog={currentRow}
          />
          <BlogsMutateDrawer
            key={`blog-edit-${currentRow.host}`}
            open={open === 'edit'}
            onOpenChange={close('edit')}
            blog={currentRow}
          />
          <BlogsDeleteDialog
            key={`blog-delete-${currentRow.host}`}
            open={open === 'delete'}
            onOpenChange={close('delete')}
            blog={currentRow}
          />
        </>
      )}
    </>
  )
}
