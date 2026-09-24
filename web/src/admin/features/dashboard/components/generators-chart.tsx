import { Bar, BarChart, ResponsiveContainer, XAxis, YAxis } from 'recharts'
import type { AdminBlog } from '@/lib/admin-types'

/** Blogs per publishing system, largest first; the long tail is summed up. */
export function GeneratorsChart({ blogs }: { blogs: AdminBlog[] }) {
  const counts = new Map<string, number>()
  for (const blog of blogs) {
    const name = blog.generator && blog.generator !== 'unknown' ? blog.generator : '未识别'
    counts.set(name, (counts.get(name) ?? 0) + 1)
  }
  const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1])
  const data = sorted.slice(0, 9).map(([name, total]) => ({ name, total }))
  const rest = sorted.slice(9).reduce((sum, [, total]) => sum + total, 0)
  if (rest > 0) data.push({ name: '其他', total: rest })

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
