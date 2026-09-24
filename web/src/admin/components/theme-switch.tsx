import { Monitor, Moon, Sun } from 'lucide-react'
import { useTheme } from '@/admin/context/theme-provider'
import { Button } from '@/admin/components/ui/button'

// The reader site's toggle: the same order, icons and wording.
const MODES = {
  system: { icon: Monitor, label: '自动模式', next: 'dark' },
  dark: { icon: Moon, label: '深色模式', next: 'light' },
  light: { icon: Sun, label: '浅色模式', next: 'system' },
} as const

/** Cycles automatic, dark and light mode, showing the current one. */
export function ThemeSwitch() {
  const { theme, setTheme } = useTheme()
  const { icon: Icon, label, next } = MODES[theme]
  const hint = `${label} · 点击切换至${MODES[next].label}`

  return (
    <Button
      variant='ghost'
      size='icon'
      className='scale-95 rounded-full'
      aria-label={hint}
      title={hint}
      onClick={() => setTheme(next)}
    >
      <Icon className='size-[1.2rem]' />
    </Button>
  )
}
