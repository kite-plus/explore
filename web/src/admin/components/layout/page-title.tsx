/** A page's heading row, as each shadcn-admin page writes it. */
export function PageTitle({
  title,
  description,
  children,
}: {
  title: string
  description: string
  children?: React.ReactNode
}) {
  return (
    <div className='flex flex-wrap items-end justify-between gap-2'>
      <div>
        <h2 className='text-2xl font-bold tracking-tight'>{title}</h2>
        <p className='text-muted-foreground'>{description}</p>
      </div>
      {children && <div className='flex gap-2'>{children}</div>}
    </div>
  )
}
