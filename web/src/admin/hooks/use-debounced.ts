import { useEffect, useState } from 'react'

export function useDebounced<T>(value: T, delay = 300): T {
  const [current, setCurrent] = useState(value)
  useEffect(() => {
    const timer = window.setTimeout(() => setCurrent(value), delay)
    return () => window.clearTimeout(timer)
  }, [value, delay])
  return current
}
