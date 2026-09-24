import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { toastError } from '@/admin/lib/api'
import { cn } from '@/admin/lib/utils'
import { Button } from '@/admin/components/ui/button'

/** Refetches every admin query on the page and toasts the outcome. */
export function RefreshButton() {
  const queryClient = useQueryClient()
  const [refreshing, setRefreshing] = useState(false)

  const refresh = async () => {
    if (refreshing) return
    setRefreshing(true)
    const started = performance.now()
    let failure: unknown = null
    try {
      await queryClient.invalidateQueries({ queryKey: ['admin'] }, { throwOnError: true })
    } catch (cause) {
      failure = cause
    }
    // animate-spin turns once a second. Finishing the turn keeps the icon
    // from jumping back, and shows a spin even when the answer is instant.
    const turn = 1000 - ((performance.now() - started) % 1000)
    await new Promise((resolve) => setTimeout(resolve, turn))
    setRefreshing(false)
    if (failure) toastError(failure, '刷新失败，请重试。')
    else toast.success('数据已刷新')
  }

  return (
    <Button variant='outline' aria-busy={refreshing} onClick={() => void refresh()}>
      <RefreshCw className={cn(refreshing && 'animate-spin')} />
      刷新
    </Button>
  )
}
