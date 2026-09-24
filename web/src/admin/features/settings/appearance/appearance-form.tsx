import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { useTheme } from '@/admin/context/theme-provider'
import { Button } from '@/admin/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/admin/components/ui/form'
import { RadioGroup, RadioGroupItem } from '@/admin/components/ui/radio-group'

const appearanceFormSchema = z.object({
  theme: z.enum(['light', 'dark', 'system']),
})
type AppearanceFormValues = z.infer<typeof appearanceFormSchema>

function LightPreview() {
  return (
    <div className='space-y-2 rounded-sm bg-[#ecedef] p-2'>
      <div className='space-y-2 rounded-md bg-white p-2 shadow-xs'>
        <div className='h-2 w-20 rounded-lg bg-[#ecedef]' />
        <div className='h-2 w-25 rounded-lg bg-[#ecedef]' />
      </div>
      <div className='flex items-center space-x-2 rounded-md bg-white p-2 shadow-xs'>
        <div className='h-4 w-4 rounded-full bg-[#ecedef]' />
        <div className='h-2 w-25 rounded-lg bg-[#ecedef]' />
      </div>
      <div className='flex items-center space-x-2 rounded-md bg-white p-2 shadow-xs'>
        <div className='h-4 w-4 rounded-full bg-[#ecedef]' />
        <div className='h-2 w-25 rounded-lg bg-[#ecedef]' />
      </div>
    </div>
  )
}

function DarkPreview() {
  return (
    <div className='space-y-2 rounded-sm bg-slate-950 p-2'>
      <div className='space-y-2 rounded-md bg-slate-800 p-2 shadow-xs'>
        <div className='h-2 w-20 rounded-lg bg-slate-400' />
        <div className='h-2 w-25 rounded-lg bg-slate-400' />
      </div>
      <div className='flex items-center space-x-2 rounded-md bg-slate-800 p-2 shadow-xs'>
        <div className='h-4 w-4 rounded-full bg-slate-400' />
        <div className='h-2 w-25 rounded-lg bg-slate-400' />
      </div>
      <div className='flex items-center space-x-2 rounded-md bg-slate-800 p-2 shadow-xs'>
        <div className='h-4 w-4 rounded-full bg-slate-400' />
        <div className='h-2 w-25 rounded-lg bg-slate-400' />
      </div>
    </div>
  )
}

const OPTIONS = [
  { value: 'light' as const, label: '浅色', preview: <LightPreview /> },
  { value: 'dark' as const, label: '深色', preview: <DarkPreview /> },
  {
    value: 'system' as const,
    label: '跟随系统',
    preview: (
      <div className='grid grid-cols-2 overflow-hidden rounded-sm'>
        <div className='overflow-hidden'>
          <LightPreview />
        </div>
        <div className='overflow-hidden'>
          <DarkPreview />
        </div>
      </div>
    ),
  },
]

export function AppearanceForm() {
  const { theme, setTheme } = useTheme()
  const form = useForm<AppearanceFormValues>({
    resolver: zodResolver(appearanceFormSchema),
    defaultValues: { theme },
  })

  function onSubmit(data: AppearanceFormValues) {
    setTheme(data.theme)
    form.reset(data)
    toast.success('外观已更新')
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-8'>
        <FormField
          control={form.control}
          name='theme'
          render={({ field }) => (
            <FormItem>
              <FormLabel>主题</FormLabel>
              <FormDescription>前台和后台共用这个选择，只保存在这台设备上。</FormDescription>
              <FormMessage />
              <RadioGroup
                onValueChange={field.onChange}
                value={field.value}
                className='grid max-w-2xl grid-cols-1 gap-8 pt-2 sm:grid-cols-3'
              >
                {OPTIONS.map((option) => (
                  <FormItem key={option.value}>
                    <FormLabel className='flex-col items-stretch [&:has([data-state=checked])>div]:border-primary'>
                      <FormControl>
                        <RadioGroupItem value={option.value} className='sr-only' />
                      </FormControl>
                      <div className='items-center rounded-md border-2 border-muted p-1 hover:border-accent'>
                        {option.preview}
                      </div>
                      <span className='block w-full p-2 text-center font-normal'>{option.label}</span>
                    </FormLabel>
                  </FormItem>
                ))}
              </RadioGroup>
            </FormItem>
          )}
        />
        <Button type='submit' disabled={!form.formState.isDirty}>
          保存外观
        </Button>
      </form>
    </Form>
  )
}
