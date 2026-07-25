import { useState } from 'react'
import { motion } from 'framer-motion'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError, type AdminUser, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'

export default function AdminUsers({ user }: { user: User }) {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['users'], queryFn: api.users })
  const [editing, setEditing] = useState<number | null>(null)
  const [error, setError] = useState('')

  async function removeUser(u: AdminUser) {
    if (!window.confirm(`Удалить пользователя ${u.full_name}? Это действие необратимо.`)) return
    setError('')
    try {
      await api.deleteUser(u.id)
      await queryClient.invalidateQueries({ queryKey: ['users'] })
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Не удалось удалить')
    }
  }

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />
      <main className="mx-auto max-w-4xl px-5 pt-8 pb-24">
        <h1 className="mb-1 text-[24px] font-extrabold tracking-tight">Пользователи</h1>
        <p className="mb-6 text-sm" style={{ color: C.muted }}>{data?.users.length ?? '…'} аккаунтов</p>

        {error && (
          <div className="mb-4 rounded-xl px-4 py-3 text-sm font-semibold" role="alert" style={{ background: C.errBg, color: C.err }}>
            {error}
          </div>
        )}

        {isLoading ? (
          <div className="grid gap-2.5">
            {[1, 2, 3].map((i) => <div key={i} className="h-20 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />)}
          </div>
        ) : (
          <div className="grid gap-2.5">
            {data?.users.map((u) => (
              <motion.article
                key={u.id}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                className="rounded-2xl border p-4" style={{ background: C.surface, borderColor: C.line }}
              >
                <div className="flex flex-wrap items-center gap-3">
                  <span className="grid size-10 place-items-center rounded-full text-[13px] font-extrabold text-white"
                        style={{ background: u.role === 'admin' ? C.accent : C.muted }}>
                    {(u.full_name[0] || '?').toUpperCase()}
                  </span>
                  <div className="min-w-0">
                    <div className="font-bold">
                      {u.full_name}
                      {u.id === user.id && <span className="ml-2 text-[11px] font-bold" style={{ color: C.faint }}>(вы)</span>}
                    </div>
                    <div className="truncate text-[13px]" style={{ color: C.muted }}>{u.email}</div>
                  </div>
                  <span className="rounded-full px-2.5 py-1 text-[11px] font-extrabold uppercase"
                        style={u.role === 'admin'
                          ? { background: C.accentSoft, color: C.accentDark }
                          : { background: C.track, color: C.muted }}>
                    {u.role === 'admin' ? 'админ' : 'инспектор'}
                  </span>
                  <span className="ml-auto text-[12.5px] whitespace-nowrap tnum" style={{ color: C.faint }}>
                    {u.acts} актов · с {u.created}
                  </span>
                  <button onClick={() => { setEditing(editing === u.id ? null : u.id); setError('') }}
                          className="cursor-pointer rounded-full border px-3.5 py-1.5 text-[12.5px] font-bold"
                          style={{ borderColor: C.line, color: C.accentDark }}>
                    {editing === u.id ? 'Свернуть' : 'Изменить'}
                  </button>
                  {u.id !== user.id && (
                    <button onClick={() => removeUser(u)}
                            className="cursor-pointer rounded-full border px-3.5 py-1.5 text-[12.5px] font-bold"
                            style={{ borderColor: C.line, color: C.err }}>
                      Удалить
                    </button>
                  )}
                </div>

                {editing === u.id && (
                  <EditUserPanel
                    u={u}
                    self={u.id === user.id}
                    onDone={async () => {
                      setEditing(null)
                      await queryClient.invalidateQueries({ queryKey: ['users'] })
                    }}
                  />
                )}
              </motion.article>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}

function EditUserPanel({ u, self, onDone }: { u: AdminUser; self: boolean; onDone: () => void }) {
  const [fullName, setFullName] = useState(u.full_name)
  const [email, setEmail] = useState(u.email)
  const [role, setRole] = useState(u.role)
  const [newPassword, setNewPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  async function save() {
    setBusy(true)
    setErr('')
    try {
      await api.updateUser(u.id, { full_name: fullName, email, role, new_password: newPassword || undefined })
      onDone()
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : 'Не удалось сохранить')
      setBusy(false)
    }
  }

  const input = 'w-full rounded-[10px] border px-3 py-2.5 text-[14px] outline-none'
  const style = { background: C.surface, borderColor: C.line, color: C.ink }

  return (
    <motion.div
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: 'auto' }}
      className="overflow-hidden"
    >
      <div className="mt-4 grid gap-3 border-t pt-4 sm:grid-cols-2" style={{ borderColor: C.line }}>
        <input value={fullName} onChange={(e) => setFullName(e.target.value)} placeholder="ФИО" aria-label="ФИО" className={input} style={style} />
        <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="email" aria-label="Email" className={input} style={style} />
        <select value={role} onChange={(e) => setRole(e.target.value as AdminUser['role'])} disabled={self}
                aria-label="Роль" className={input} style={style}>
          <option value="inspector">инспектор</option>
          <option value="admin">админ</option>
        </select>
        <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)}
               placeholder="новый пароль (не обязательно)" aria-label="Новый пароль" className={input} style={style} />
      </div>
      {err && <div className="mt-2 text-[13px] font-semibold" style={{ color: C.err }}>{err}</div>}
      <button onClick={save} disabled={busy}
              className="mt-3 cursor-pointer rounded-full px-5 py-2 text-[13px] font-extrabold text-white disabled:opacity-50"
              style={{ background: C.accent }}>
        {busy ? 'Сохраняем…' : 'Сохранить'}
      </button>
    </motion.div>
  )
}
