import { useEffect, useState } from 'react'
import { Copy, KeyRound, Loader2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '@/admin/components/ui/alert'
import { Button } from '@/admin/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/admin/components/ui/dialog'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import type { AdminUserRow } from '@/lib/admin-types'
import { useUserActions } from '../api'

const MIN_PASSWORD = 12
const MAX_PASSWORD_BYTES = 72

// No 0/O or 1/l/I, so the password survives being read out or retyped.
const ALPHABET = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789'

/** Sixteen random characters in groups of four, about 92 bits. */
function generatePassword() {
  const chars = Array.from(crypto.getRandomValues(new Uint32Array(16)), (n) => ALPHABET[n % ALPHABET.length])
  return [0, 4, 8, 12].map((i) => chars.slice(i, i + 4).join('')).join('-')
}

function passwordProblem(password: string) {
  if (password.length < MIN_PASSWORD) return `至少 ${MIN_PASSWORD} 位。`
  if (new TextEncoder().encode(password).length > MAX_PASSWORD_BYTES) return '太长了，最多 72 个英文字符或 24 个汉字。'
  return ''
}

function copy(password: string) {
  navigator.clipboard.writeText(password).then(
    () => toast.success('已复制临时密码'),
    () => toast.error('复制失败，请手动选中复制')
  )
}

type ResetPasswordDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: AdminUserRow
  onDone: () => void
}

export function ResetPasswordDialog({ open, onOpenChange, user, onDone }: ResetPasswordDialogProps) {
  const { resetPassword } = useUserActions()
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [reset, setReset] = useState(false)
  const problem = passwordProblem(password)

  useEffect(() => {
    if (!open) return
    setPassword(generatePassword())
    setReset(false)
  }, [open])

  async function submit(event: React.FormEvent) {
    event.preventDefault()
    if (problem || busy) return
    setBusy(true)
    const ok = await resetPassword(user, password)
    setBusy(false)
    if (ok) setReset(true)
  }

  return (
    <Dialog open={open} onOpenChange={(state) => !busy && (reset ? onDone() : onOpenChange(state))}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader className='text-start'>
          <DialogTitle>{reset ? '密码已重置' : '重置密码'}</DialogTitle>
          <DialogDescription>
            {user.display_name}（{user.email}），ID {user.number}
          </DialogDescription>
        </DialogHeader>
        {reset ? (
          <div className='space-y-4'>
            <p className='text-sm text-muted-foreground'>
              这个账号在所有设备上都已退出登录。把下面的临时密码告诉对方，对方用它登录后，账号页会提示改成自己的密码。
            </p>
            <div className='flex gap-2'>
              <Input readOnly value={password} className='font-mono' onFocus={(event) => event.target.select()} />
              <Button type='button' variant='outline' onClick={() => copy(password)}>
                <Copy />
                复制
              </Button>
            </div>
            <p className='text-xs text-muted-foreground'>关闭后不会再显示这个密码。</p>
          </div>
        ) : (
          <form id='reset-password-form' onSubmit={(event) => void submit(event)} className='space-y-4 px-0.5'>
            <Alert>
              <KeyRound />
              <AlertDescription>
                Explore 没有自助找回密码。请先通过你们约定的渠道核实对方就是这个账号的主人，再重置。重置后该账号所有设备退出登录。
              </AlertDescription>
            </Alert>
            <div className='space-y-2'>
              <Label htmlFor='temporary-password'>临时密码</Label>
              <div className='flex gap-2'>
                <Input
                  id='temporary-password'
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  className='font-mono'
                  autoComplete='off'
                  spellCheck={false}
                  aria-invalid={Boolean(problem)}
                />
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  aria-label='换一个'
                  title='换一个'
                  onClick={() => setPassword(generatePassword())}
                >
                  <RefreshCw />
                </Button>
                <Button type='button' variant='outline' size='icon' aria-label='复制' title='复制' onClick={() => copy(password)}>
                  <Copy />
                </Button>
              </div>
              <p className={problem ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}>
                {problem || '已随机生成，也可以改成你想给的密码。'}
              </p>
            </div>
          </form>
        )}
        <DialogFooter>
          {reset ? (
            <Button onClick={onDone}>完成</Button>
          ) : (
            <Button type='submit' form='reset-password-form' variant='destructive' disabled={busy || Boolean(problem)}>
              {busy && <Loader2 className='animate-spin' />}
              重置密码
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
