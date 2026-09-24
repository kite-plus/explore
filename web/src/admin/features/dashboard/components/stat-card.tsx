import { type LucideIcon } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/admin/components/ui/card'
import { Skeleton } from '@/admin/components/ui/skeleton'

type StatCardProps = {
  title: string
  value: number | string | undefined
  hint: React.ReactNode
  icon: LucideIcon
}

export function StatCard({ title, value, hint, icon: Icon }: StatCardProps) {
  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='text-sm font-medium'>{title}</CardTitle>
        <Icon className='h-4 w-4 text-muted-foreground' />
      </CardHeader>
      <CardContent>
        {value === undefined ? (
          <Skeleton className='h-8 w-16' />
        ) : (
          <div className='text-2xl font-bold tabular-nums'>
            {typeof value === 'number' ? value.toLocaleString('zh-CN') : value}
          </div>
        )}
        <p className='text-xs text-muted-foreground'>{hint}</p>
      </CardContent>
    </Card>
  )
}
