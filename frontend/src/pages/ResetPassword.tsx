import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, ApiError } from '../lib/api'
import {
  AuthShell,
  Notice,
  PasswordField,
  cardClass,
  fieldClass,
  hintClass,
  labelClass,
  linkClass,
  submitClass,
} from '../components/AuthShell'

export default function ResetPassword() {
  const [params] = useSearchParams()
  const [email, setEmail] = useState(params.get('email') ?? '')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()

  const mismatch = confirm !== '' && password !== confirm

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (code.length !== 6) {
      setError('Введите 6-значный код из письма')
      return
    }
    if (password !== confirm) {
      setError('Пароли не совпадают')
      return
    }
    setBusy(true)
    try {
      await api.resetPassword({ email: email.trim(), code, password, confirm })
      navigate('/login?reset=1', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось сменить пароль. Попробуйте ещё раз.')
      setBusy(false)
    }
  }

  return (
    <AuthShell
      subtitle="Введите код из письма и новый пароль"
      footer={
        <>
          <Link to="/forgot-password" className={linkClass}>
            Запросить код заново
          </Link>
          <span className="mx-2 text-faint">·</span>
          <Link to="/login" className={linkClass}>
            Вернуться ко входу
          </Link>
        </>
      }
    >
      <form onSubmit={submit} className={cardClass}>
        <label className="mb-4 block">
          <span className={labelClass}>Email</span>
          <input
            type="email"
            required
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="example@mail.ru"
            className={fieldClass}
          />
        </label>

        <label className="mb-4 block">
          <span className={labelClass}>Код из письма</span>
          <input
            type="text"
            required
            inputMode="numeric"
            maxLength={6}
            autoComplete="one-time-code"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            placeholder="000000"
            className={`${fieldClass} tnum font-mono tracking-[0.3em]`}
          />
          <span className={hintClass}>6 цифр, код действует 15 минут</span>
        </label>

        <PasswordField
          label="Новый пароль"
          autoComplete="new-password"
          placeholder="Минимум 6 символов"
          value={password}
          onChange={setPassword}
          hint={<span className={hintClass}>Минимум 6 символов, заглавная буква, цифра и спецсимвол</span>}
        />

        <PasswordField
          label="Повторите пароль"
          className="mb-6"
          autoComplete="new-password"
          placeholder="Повторите пароль"
          value={confirm}
          onChange={setConfirm}
          hint={mismatch && <span className="mt-1.5 block text-xs font-semibold text-err">Пароли не совпадают</span>}
        />

        {error && <Notice kind="err">{error}</Notice>}

        <motion.button type="submit" disabled={busy} whileTap={{ scale: 0.98 }} className={submitClass}>
          {busy ? 'Сохраняем…' : 'Сменить пароль'}
        </motion.button>
      </form>
    </AuthShell>
  )
}
