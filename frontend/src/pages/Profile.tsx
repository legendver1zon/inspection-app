import { useRef, useState } from 'react'
import { motion } from 'framer-motion'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'
import SignaturePad from './edit/SignaturePad'

export default function Profile({ user }: { user: User }) {
  const queryClient = useQueryClient()
  const [fullName, setFullName] = useState(user.full_name)
  const [initials, setInitials] = useState(user.initials)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const avatarInput = useRef<HTMLInputElement>(null)
  const [padOpen, setPadOpen] = useState(false)
  const [sigBusy, setSigBusy] = useState(false)

  async function saveSignature(dataUrl: string) {
    setPadOpen(false)
    setSigBusy(true)
    try {
      const { user: fresh } = await api.setProfileSignature(dataUrl)
      queryClient.setQueryData(['me'], { user: fresh })
      setMsg({ ok: true, text: 'Подпись сохранена' })
    } catch (e) {
      setMsg({ ok: false, text: e instanceof ApiError ? e.message : 'Не удалось сохранить подпись' })
    } finally {
      setSigBusy(false)
    }
  }
  async function removeSignature() {
    if (!window.confirm('Удалить подпись из профиля? В уже подписанных актах она останется.')) return
    setSigBusy(true)
    try {
      const { user: fresh } = await api.deleteProfileSignature()
      queryClient.setQueryData(['me'], { user: fresh })
    } finally {
      setSigBusy(false)
    }
  }

  async function save() {
    setBusy(true)
    setMsg(null)
    try {
      const { user: fresh } = await api.updateProfile({
        full_name: fullName,
        initials,
        current_password: currentPassword || undefined,
        new_password: newPassword || undefined,
        confirm: confirm || undefined,
      })
      queryClient.setQueryData(['me'], { user: fresh })
      setCurrentPassword(''); setNewPassword(''); setConfirm('')
      setMsg({ ok: true, text: 'Профиль обновлён' })
    } catch (e) {
      setMsg({ ok: false, text: e instanceof ApiError ? e.message : 'Не удалось сохранить' })
    } finally {
      setBusy(false)
    }
  }

  async function changeAvatar(files: FileList | null) {
    const file = files?.[0]
    if (!file) return
    await api.uploadAvatar(file)
    await queryClient.invalidateQueries({ queryKey: ['me'] })
    if (avatarInput.current) avatarInput.current.value = ''
  }

  const label = 'mb-1.5 block text-xs font-bold tracking-wide uppercase'
  const input = 'w-full rounded-xl border px-4 py-3 text-[15px] outline-none focus:ring-2'
  const inputStyle = { background: C.surface, borderColor: C.line, color: C.ink, ['--tw-ring-color' as string]: C.accentSoft }

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />
      <main className="mx-auto max-w-xl px-5 pt-8 pb-24">
        <h1 className="mb-6 text-[24px] font-extrabold tracking-tight">Профиль</h1>

        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="grid gap-5">
          {/* Аватар */}
          <section className="flex items-center gap-4 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
            {user.avatar_url ? (
              <img src={user.avatar_url} alt="" className="size-16 rounded-full border object-cover" style={{ borderColor: C.line }} />
            ) : (
              <span className="grid size-16 place-items-center rounded-full text-xl font-extrabold text-white" style={{ background: C.accent }}>
                {(user.full_name[0] || '?').toUpperCase()}
              </span>
            )}
            <div>
              <div className="font-bold">{user.full_name}</div>
              <div className="text-sm" style={{ color: C.muted }}>{user.email} · {user.role === 'admin' ? 'администратор' : 'инспектор'}</div>
              <button onClick={() => avatarInput.current?.click()}
                      className="mt-1.5 cursor-pointer text-[13px] font-bold hover:underline" style={{ color: C.accentDark }}>
                Сменить фото
              </button>
              <input ref={avatarInput} type="file" accept="image/jpeg,image/png,image/webp" hidden
                     onChange={(e) => changeAvatar(e.target.files)} />
            </div>
          </section>

          {/* Данные */}
          <section className="grid gap-4 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
            <label>
              <span className={label} style={{ color: C.muted }}>ФИО</span>
              <input value={fullName} onChange={(e) => setFullName(e.target.value)} className={input} style={inputStyle} />
            </label>
            <label>
              <span className={label} style={{ color: C.muted }}>Инициалы (в актах)</span>
              <input value={initials} onChange={(e) => setInitials(e.target.value)} className={input} style={inputStyle} />
            </label>
          </section>

          {/* Подпись */}
          <section className="grid gap-3 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
            <h2 className="text-[13px] font-extrabold tracking-wide uppercase" style={{ color: C.muted }}>Моя подпись</h2>
            {user.has_signature ? (
              <img src={user.signature_url} alt="Подпись" className="h-20 w-fit max-w-full rounded-xl border object-contain px-3" style={{ borderColor: C.line, background: '#fff' }} />
            ) : (
              <p className="text-sm" style={{ color: C.muted }}>Подпись не задана. Нарисуйте её один раз, чтобы ставить в акты одним нажатием.</p>
            )}
            <div className="flex flex-wrap gap-2">
              <button onClick={() => setPadOpen(true)} disabled={sigBusy}
                      className="cursor-pointer rounded-full px-5 py-2.5 text-[13.5px] font-extrabold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                      style={{ background: C.accent }}>
                {user.has_signature ? 'Заменить' : 'Нарисовать подпись'}
              </button>
              {user.has_signature && (
                <button onClick={removeSignature} disabled={sigBusy}
                        className="cursor-pointer rounded-full border px-5 py-2.5 text-[13.5px] font-bold transition-colors disabled:opacity-50"
                        style={{ borderColor: C.line, color: C.err }}>
                  Удалить
                </button>
              )}
            </div>
          </section>
          <SignaturePad open={padOpen} title="Моя подпись" onClose={() => setPadOpen(false)} onDone={saveSignature} />

          {/* Пароль */}
          <section className="grid gap-4 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
            <h2 className="text-[13px] font-extrabold tracking-wide uppercase" style={{ color: C.muted }}>Смена пароля</h2>
            <label>
              <span className={label} style={{ color: C.muted }}>Текущий пароль</span>
              <input type="password" autoComplete="current-password" value={currentPassword}
                     onChange={(e) => setCurrentPassword(e.target.value)} className={input} style={inputStyle} />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <label>
                <span className={label} style={{ color: C.muted }}>Новый пароль</span>
                <input type="password" autoComplete="new-password" value={newPassword}
                       onChange={(e) => setNewPassword(e.target.value)} className={input} style={inputStyle} />
              </label>
              <label>
                <span className={label} style={{ color: C.muted }}>Ещё раз</span>
                <input type="password" autoComplete="new-password" value={confirm}
                       onChange={(e) => setConfirm(e.target.value)} className={input} style={inputStyle} />
              </label>
            </div>
            <p className="text-[12px]" style={{ color: C.faint }}>
              Минимум 6 символов, заглавная буква, цифра и спецсимвол. Оставьте пустым, чтобы не менять.
            </p>
          </section>

          {msg && (
            <div className="rounded-xl px-4 py-3 text-sm font-semibold" role="status"
                 style={{ background: msg.ok ? C.okBg : C.errBg, color: msg.ok ? C.ok : C.err }}>
              {msg.text}
            </div>
          )}

          <motion.button
            whileTap={{ scale: 0.98 }}
            onClick={save}
            disabled={busy}
            className="cursor-pointer justify-self-start rounded-full px-6 py-3 text-[14px] font-extrabold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
            style={{ background: C.accent }}
          >
            {busy ? 'Сохраняем…' : 'Сохранить'}
          </motion.button>
        </motion.div>
      </main>
    </div>
  )
}
