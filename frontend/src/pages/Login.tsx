import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '../lib/api'
import { uploadQueue } from '../lib/uploadQueue'
import {
  AuthShell,
  Notice,
  PasswordField,
  cardClass,
  fieldClass,
  labelClass,
  linkClass,
  submitClass,
} from '../components/AuthShell'

export default function Login() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from
  const queryClient = useQueryClient()

  const notice =
    params.get('registered') === '1'
      ? 'Регистрация завершена, войдите'
      : params.get('reset') === '1'
        ? 'Пароль изменён, войдите'
        : ''

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const { user } = await api.login(email, password)
      queryClient.setQueryData(['me'], { user })
      uploadQueue.resume()
      navigate(from && from !== '/login' ? from : '/inspections', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось войти. Попробуйте ещё раз.')
      setBusy(false)
    }
  }

  return (
    <AuthShell
      subtitle="Акты осмотра объектов недвижимости"
      footer={
        <>
          <Link to="/register" className={linkClass}>
            Регистрация
          </Link>
          <span className="mx-2 text-faint">·</span>
          <Link to="/forgot-password" className={linkClass}>
            Забыли пароль?
          </Link>
        </>
      }
    >
      <form onSubmit={submit} className={cardClass}>
        {notice && !error && <Notice kind="ok">{notice}</Notice>}

        <label className="mb-4 block">
          <span className={labelClass}>Email</span>
          <input
            type="email"
            required
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            className={fieldClass}
          />
        </label>

        <PasswordField
          label="Пароль"
          className="mb-6"
          autoComplete="current-password"
          value={password}
          onChange={setPassword}
        />

        {error && <Notice kind="err">{error}</Notice>}

        <motion.button type="submit" disabled={busy} whileTap={{ scale: 0.98 }} className={submitClass}>
          {busy ? 'Входим…' : 'Войти'}
        </motion.button>
      </form>
    </AuthShell>
  )
}
