import { useRef, useState } from 'react'
import { X } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import { Badge } from '@/admin/components/ui/badge'

type TagInputProps = {
  id?: string
  value: string[]
  onChange: (value: string[]) => void
  placeholder?: string
  max?: number
  className?: string
}

/** Free-form values such as domains: Enter, space or comma adds one, Backspace removes the last. */
export function TagInput({
  id,
  value,
  onChange,
  placeholder = '输入后按 Enter 添加',
  max = 20,
  className,
}: TagInputProps) {
  const [input, setInput] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const full = value.length >= max

  function add(raw: string) {
    const tag = raw.trim().toLowerCase()
    setInput('')
    if (!tag || value.includes(tag) || full) return
    onChange([...value, tag])
  }

  return (
    <div
      className={cn(
        'flex min-h-9 w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-transparent px-2 py-1.5 text-sm shadow-xs transition-[color,box-shadow] dark:bg-input/30',
        'focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50',
        className
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((tag) => (
        <Badge key={tag} variant='secondary' className='gap-1 pe-1 font-mono'>
          {tag}
          <button
            type='button'
            onClick={() => onChange(value.filter((item) => item !== tag))}
            className='rounded-sm text-muted-foreground hover:text-foreground'
            aria-label={`删除 ${tag}`}
          >
            <X className='size-3' />
          </button>
        </Badge>
      ))}
      {!full && (
        <input
          id={id}
          ref={inputRef}
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ' || event.key === ',') {
              event.preventDefault()
              add(input)
            } else if (event.key === 'Backspace' && !input && value.length > 0) {
              onChange(value.slice(0, -1))
            }
          }}
          onBlur={() => add(input)}
          placeholder={value.length === 0 ? placeholder : ''}
          className='h-6 min-w-28 flex-1 bg-transparent outline-none placeholder:text-muted-foreground'
        />
      )}
    </div>
  )
}
