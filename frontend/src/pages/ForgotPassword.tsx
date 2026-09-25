import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { Link, useNavigate } from 'react-router-dom'
import { api, ApiError } from '../lib/api'
import { AuthShell, Notice, cardClass, fieldClass, labelClass, linkClass, submitClass } from '../components/AuthShell'

export default function ForgotPassword() {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api.forgotPassword(email.trim())
      setSent(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось отправить код. Попробуйте ещё раз.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthShell
      subtitle="Введите email — пришлём код для сброса"
      footer={
        <Link to="/login" className={linkClass}>
          Вернуться ко входу
        </Link>
      }
    >
      {sent ? (
        <div className={cardClass}>
          <Notice kind="ok">Код отправлен на {email.trim()}, действителен 15 минут</Notice>
          <p className="mb-6 text-sm text-muted">
            Если аккаунт с таким email существует — письмо с кодом уже отправлено. Проверьте входящие и папку «Спам».
          </p>
          <motion.button
            type="button"
            whileTap={{ scale: 0.98 }}
            onClick={() => navigate(`/reset-password?email=${encodeURIComponent(email.trim())}`)}
            className={submitClass}
          >
            Ввести код
          </motion.button>
        </div>
      ) : (
        <form onSubmit={submit} className={cardClass}>
          <label className="mb-6 block">
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

          {error && <Notice kind="err">{error}</Notice>}

          <motion.button type="submit" disabled={busy} whileTap={{ scale: 0.98 }} className={submitClass}>
            {busy ? 'Отправляем…' : 'Отправить код'}
          </motion.button>
        </form>
      )}
    </AuthShell>
  )
}
