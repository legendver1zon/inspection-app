import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../lib/api'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const { user } = await api.login(email, password)
      queryClient.setQueryData(['me'], { user })
      navigate('/inspections', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось войти. Попробуйте ещё раз.')
      setBusy(false)
    }
  }

  return (
    <motion.main
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="grid min-h-dvh place-items-center bg-paper px-4"
    >
      <motion.div
        initial={{ opacity: 0, y: 18, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ type: 'spring', stiffness: 260, damping: 24 }}
        className="w-full max-w-md"
      >
        <div className="mb-8 flex flex-col items-center gap-3">
          <motion.div
            initial={{ rotate: -12, scale: 0.8 }}
            animate={{ rotate: 0, scale: 1 }}
            transition={{ type: 'spring', stiffness: 300, damping: 18, delay: 0.05 }}
            className="grid size-14 place-items-center rounded-2xl bg-gradient-to-br from-accent-bright to-accent text-2xl font-black text-white shadow-[0_8px_24px_rgba(62,104,168,.35)]"
          >
            А
          </motion.div>
          <h1 className="text-2xl font-extrabold tracking-tight text-ink">АктОсмотр</h1>
          <p className="text-sm text-muted">Акты осмотра объектов недвижимости</p>
        </div>

        <form
          onSubmit={submit}
          className="rounded-3xl border border-line bg-surface p-6 shadow-[0_1px_2px_rgba(43,39,33,.05),0_16px_40px_rgba(43,39,33,.08)] sm:p-8"
        >
          <label className="mb-4 block">
            <span className="mb-1.5 block text-xs font-bold tracking-wide text-muted uppercase">Email</span>
            <input
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              className="w-full rounded-xl border border-line bg-surface px-4 py-3 text-[15px] text-ink placeholder-faint transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(62,104,168,.15)] focus:outline-none"
            />
          </label>

          <label className="mb-6 block">
            <span className="mb-1.5 block text-xs font-bold tracking-wide text-muted uppercase">Пароль</span>
            <div className="relative">
              <input
                type={showPassword ? 'text' : 'password'}
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full rounded-xl border border-line bg-surface px-4 py-3 pr-12 text-[15px] text-ink placeholder-faint transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(62,104,168,.15)] focus:outline-none"
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}
                className="absolute top-1/2 right-3 -translate-y-1/2 cursor-pointer p-1 text-faint transition-colors hover:text-muted"
              >
                <EyeIcon crossed={showPassword} />
              </button>
            </div>
          </label>

          {error && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              className="mb-4 overflow-hidden rounded-xl bg-err-bg px-4 py-3 text-sm font-semibold whitespace-pre-line text-err"
              role="alert"
            >
              {error}
            </motion.div>
          )}

          <motion.button
            type="submit"
            disabled={busy}
            whileTap={{ scale: 0.98 }}
            className="w-full cursor-pointer rounded-xl bg-accent py-3.5 text-[15px] font-extrabold text-on-accent shadow-[0_5px_16px_rgba(62,104,168,.28)] transition-colors hover:bg-accent-hover disabled:opacity-60"
          >
            {busy ? 'Входим…' : 'Войти'}
          </motion.button>
        </form>

        <p className="mt-6 text-center text-sm text-muted">
          Нет аккаунта или забыли пароль? Обратитесь к администратору.
        </p>
      </motion.div>
    </motion.main>
  )
}

function EyeIcon({ crossed }: { crossed: boolean }) {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M2 12s3.5-6.5 10-6.5S22 12 22 12s-3.5 6.5-10 6.5S2 12 2 12Z" />
      <circle cx="12" cy="12" r="2.8" />
      {crossed && <path d="m4 4 16 16" />}
    </svg>
  )
}
