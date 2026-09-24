import { CircleAlert, CircleCheck, Info, TriangleAlert } from 'lucide-react'
import { Toaster as Sonner, type ToasterProps } from 'sonner'
import { useTheme } from '@/admin/context/theme-provider'

/**
 * shadcn-admin's Toaster, dressed as mq-studio dresses its own: each state has
 * an icon, which admin.css tints under .admin-toaster, and the close button
 * sits on the top-right corner.
 */
export function Toaster({ ...props }: ToasterProps) {
  const { theme = 'system' } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps['theme']}
      className='admin-toaster toaster group [&_div[data-content]]:w-full'
      position='top-center'
      offset={16}
      gap={8}
      visibleToasts={3}
      closeButton
      toastOptions={{ closeButtonAriaLabel: '关闭' }}
      icons={{
        success: <CircleCheck size={16} />,
        info: <Info size={16} />,
        warning: <TriangleAlert size={16} />,
        error: <CircleAlert size={16} />,
      }}
      style={
        {
          '--normal-bg': 'var(--popover)',
          '--normal-text': 'var(--popover-foreground)',
          '--normal-border': 'var(--border)',
          '--border-radius': '12px',
          '--width': '340px',
          '--toast-close-button-start': 'unset',
          '--toast-close-button-end': '0',
          '--toast-close-button-transform': 'translate(35%, -35%)',
        } as React.CSSProperties
      }
      {...props}
    />
  )
}
