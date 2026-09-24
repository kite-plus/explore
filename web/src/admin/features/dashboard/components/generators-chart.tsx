import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import { generatorInfo } from '@/admin/components/generator-label'
import type { AdminBlog } from '@/lib/admin-types'

/** Blogs per publishing system, largest first. The backend names at most ten. */
export function GeneratorsChart({ blogs }: { blogs: AdminBlog[] }) {
  const counts = new Map<string, number>()
  for (const blog of blogs) {
    const name = generatorInfo(blog.generator).label
    counts.set(name, (counts.get(name) ?? 0) + 1)
  }
  const data = [...counts.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([name, total]) => ({ name, total }))

  return (
    <ResponsiveContainer width='100%' height={350}>
      <BarChart data={data}>
        <XAxis dataKey='name' stroke='#888888' fontSize={12} tickLine={false} axisLine={false} />
        <YAxis
          direction='ltr'
          stroke='#888888'
          fontSize={12}
          tickLine={false}
          axisLine={false}
          allowDecimals={false}
        />
        <Bar dataKey='total' fill='currentColor' radius={[4, 4, 0, 0]} className='fill-primary' />
      </BarChart>
    </ResponsiveContainer>
  )
}
