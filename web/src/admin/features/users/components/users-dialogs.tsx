import { useState } from 'react'
import { TriangleAlert } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/admin/components/ui/alert'
import { Input } from '@/admin/components/ui/input'
import { Label } from '@/admin/components/ui/label'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import type { AdminUserRow } from '@/lib/admin-types'
import { useUserActions } from '../api'
import { adminCopy } from '../data/data'
import { DisableDialog } from './disable-dialog'
import { EditDialog } from './edit-dialog'
import { ResetPasswordDialog } from './reset-password-dialog'
import { UserDetailDrawer } from './user-detail-drawer'
import { useUsers } from './users-provider'

function who(users: AdminUserRow[]) {
  if (users.length !== 1) return `选中的 ${users.length} 个账号`
  return (
    <span className='font-medium text-foreground'>
      {users[0].display_name}（{users[0].email}）
    </span>
  )
}

export function UsersDialogs() {
  const { open, targets, host, close, done, detailId, showDetail } = useUsers()
  const actions = useUserActions()
  const [busy, setBusy] = useState(false)
  const [confirmation, setConfirmation] = useState('')
  const user = targets[0]

  async function run(action: () => Promise<boolean>) {
    setBusy(true)
    const ok = await action()
    setBusy(false)
    if (ok) done()
  }

  const confirm = (
    dialog: typeof open,
    props: {
      title: React.ReactNode
      desc: React.ReactNode
      confirmText: string
      destructive?: boolean
      action: () => Promise<boolean>
      disabled?: boolean
      children?: React.ReactNode
    }
  ) => (
    <ConfirmDialog
      open={open === dialog}
      onOpenChange={(state) => {
        if (state || busy) return
        setConfirmation('')
        close()
      }}
      title={props.title}
      desc={<div>{props.desc}</div>}
      confirmText={props.confirmText}
      destructive={props.destructive}
      disabled={props.disabled}
      isLoading={busy}
      handleConfirm={() => void run(props.action)}
      className='sm:max-w-md'
    >
      {props.children}
    </ConfirmDialog>
  )

  const copy = adminCopy(!user?.is_admin)

  return (
    <>
      <UserDetailDrawer id={detailId} onClose={() => showDetail(null)} />
      {user && (
        <>
          <DisableDialog open={open === 'disable'} onOpenChange={(state) => !state && close()} users={targets} onDone={done} />
          <EditDialog open={open === 'edit'} onOpenChange={(state) => !state && close()} user={user} onDone={done} />
          <ResetPasswordDialog
            open={open === 'reset-password'}
            onOpenChange={(state) => !state && close()}
            user={user}
            onDone={done}
          />
          {confirm('restore', {
            title: targets.length === 1 ? '恢复账号' : `恢复 ${targets.length} 个账号`,
            desc: <>{who(targets)}<br />恢复后可以重新登录，停用原因会被清除。</>,
            confirmText: '恢复',
            action: () => actions.setDisabled(targets, false),
          })}
          {confirm('admin', {
            title: copy.title,
            desc: <>{who(targets)}<br />{copy.desc}</>,
            confirmText: copy.action,
            destructive: copy.destructive,
            action: () => actions.setAdmin(user, !user.is_admin),
          })}
          {confirm('sign-out', {
            title: targets.length === 1 ? '强制下线' : `强制 ${targets.length} 个账号下线`,
            desc: <>{who(targets)}<br />所有设备上的登录立即失效，需要重新登录。账号本身不受影响，适合怀疑会话泄露时使用。</>,
            confirmText: '强制下线',
            action: () => actions.signOut(targets),
          })}
          {confirm('release', {
            title: '解除认领',
            desc: <>{who(targets)} 将不再是 <span className='font-mono font-medium text-foreground'>{host}</span> 的认领人。<br />博客本身不受影响，之后任何人都可以重新通过 DNS 验证认领它。</>,
            confirmText: '解除认领',
            destructive: true,
            action: () => actions.release(user, host),
          })}
          {confirm('delete', {
            title: (
              <span className='text-destructive'>
                <TriangleAlert className='me-1 inline-block stroke-destructive' size={18} /> 删除账号
              </span>
            ),
            desc: (
              <>
                {who(targets)}
                <br />
                账号、会话、订阅和博客认领都会被删除，提交过的举报保留但不再关联这个账号。只想阻止登录时，请改用停用。
              </>
            ),
            confirmText: '删除',
            destructive: true,
            disabled: confirmation.trim().toLowerCase() !== user.email.toLowerCase(),
            action: async () => {
              const ok = await actions.remove(user)
              if (ok) {
                setConfirmation('')
                if (detailId === user.id) showDetail(null)
              }
              return ok
            },
            children: (
              <div className='space-y-4'>
                <Label className='flex-col items-start gap-2'>
                  输入邮箱以确认：
                  <Input
                    value={confirmation}
                    onChange={(event) => setConfirmation(event.target.value)}
                    placeholder={user.email}
                    autoComplete='off'
                  />
                </Label>
                <Alert variant='destructive'>
                  <AlertTitle>注意</AlertTitle>
                  <AlertDescription>这项操作不能撤销。</AlertDescription>
                </Alert>
              </div>
            ),
          })}
        </>
      )}
    </>
  )
}
