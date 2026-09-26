import type { User } from '../../lib/api'
import { C } from '../../lib/palette'
import { Button } from './ui'
import type { SigRole, SigState } from './form'

function fmtAt(iso?: string) {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// Строка подписи в редакторе: кто, чем подписано, кнопки подписать/убрать.
export default function SignatureRow({ role, label, name, state, user, disabled, onDraw, onFromProfile, onClear }: {
  role: SigRole
  label: string
  name: string
  state: SigState
  user: User
  disabled?: boolean
  onDraw: () => void
  onFromProfile?: () => void
  onClear: () => void
}) {
  const img = state.dataUrl ?? (state.fromProfile ? user.signature_url : state.clear ? undefined : state.saved?.url)
  const at = state.at ?? (state.clear ? undefined : state.saved?.signed_at)
  const pending = !!(state.dataUrl || state.fromProfile)
  return (
    <div className="flex flex-col gap-2 rounded-xl border p-3" style={{ background: C.bg, borderColor: C.line }}>
      <div className="flex flex-col gap-0.5">
        <span className="text-[14px] font-semibold">{label}</span>
        <span className="text-[12px]" style={{ color: C.muted }}>{name}</span>
      </div>
      {img ? (
        <div className="flex flex-wrap items-center gap-3">
          <img src={img} alt="Подпись" className="h-12 max-w-[180px] rounded-md border object-contain px-2" style={{ borderColor: C.line, background: '#fff' }} />
          <span className="text-[12px]" style={{ color: pending ? C.warn : C.ok }}>
            {pending ? 'подписано, уйдёт с сохранением' : `подписано ${fmtAt(at)}`}
          </span>
          {!disabled && (
            <span className="flex gap-1">
              <Button className="h-8 px-2 text-[12px] sm:h-8" onClick={onDraw}>Переподписать</Button>
              <Button variant="danger-text" className="h-8 px-2 text-[12px] sm:h-8" onClick={onClear}>Убрать</Button>
            </span>
          )}
        </div>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          {onFromProfile && user.has_signature ? (
            <>
              <Button variant="primary" disabled={disabled} onClick={onFromProfile}>Поставить мою подпись</Button>
              <Button variant="text" disabled={disabled} onClick={onDraw}>Нарисовать</Button>
            </>
          ) : (
            <Button variant="primary" disabled={disabled} onClick={onDraw}>Подписать</Button>
          )}
          {role === 'inspector' && !user.has_signature && (
            <span className="text-[12px]" style={{ color: C.faint }}>Сохраните подпись в профиле, чтобы ставить одним нажатием</span>
          )}
          {role === 'owner' && <span className="text-[12px]" style={{ color: C.faint }}>После подписи собственника акт закрывается для правок</span>}
        </div>
      )}
    </div>
  )
}
