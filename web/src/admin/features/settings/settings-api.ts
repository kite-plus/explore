import { useAdminQuery } from '@/admin/lib/api'
import type { SystemSetting } from '@/lib/admin-types'

export function useSettings() {
  const query = useAdminQuery<{ data: SystemSetting[] }>('/settings')
  const byKey = new Map(query.data?.data.map((setting) => [setting.key, setting]))
  return { ...query, byKey }
}
