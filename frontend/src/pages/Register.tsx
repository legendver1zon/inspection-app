import { useState, type FormEvent } from 'react'
import { motion } from 'framer-motion'
import { Link, useNavigate } from 'react-router-dom'
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

function toInitials(fullName: string) {
  const parts = fullName.trim().split(/\s+/).filter(Boolean)
  if (!parts.length) return ''
  let r = parts[0]
  for (let i = 1; i < parts.length && i <= 2; i++) {
    const ch = [...parts[i]][0]
    if (ch) r += ` ${ch}.`
  }
  return r
}

export default function Register() {
  const [email, setEmail] = useState('')
  const [fullName, setFullName] = useState('')
  const [noPatronymic, setNoPatronymic] = useState(false)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const navigate = useNavigate()

  const words = fullName.trim().split(/\s+/).filter(Boolean).length
  const preview = toInitials(fullName)
  const mismatch = confirm !== '' && password !== confirm

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (words < (noPatronymic ? 2 : 3)) {
      setError(noPatronymic ? 'Введите Фамилию и Имя' : 'Введите полное ФИО (Фамилия, Имя и Отчество)')
      return
    }
    if (password !== confirm) {
      setError('Пароли не совпадают')
      return
    }
    setBusy(true)
    try {
      await api.register({
        email: email.trim(),
        password,
        confirm_password: confirm,
        full_name: fullName.trim(),
        no_patronymic: noPatronymic,
      })
      navigate('/login?registered=1', { replace: true })
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось создать аккаунт. Попробуйте ещё раз.')
      setBusy(false)
    }
  }

  return (
    <AuthShell
      subtitle="Создание аккаунта"
      footer={
        <>
          Уже есть аккаунт?{' '}
          <Link to="/login" className={linkClass}>
            Войти
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

        <div className="mb-4">
          <label className="block">
            <span className={labelClass}>ФИО полностью</span>
            <input
              type="text"
              required
              autoComplete="name"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              placeholder={noPatronymic ? 'Иванов Иван' : 'Иванов Иван Иванович'}
              className={fieldClass}
            />
            <span className={hintClass}>
              {preview
                ? `В подписи: ${preview}`
                : noPatronymic
                  ? 'Фамилия и имя через пробел'
                  : 'Фамилия, имя и отчество через пробел'}
            </span>
          </label>
          <label className="mt-2 flex cursor-pointer items-center gap-2 text-sm text-muted">
            <input
              type="checkbox"
              checked={noPatronymic}
              onChange={(e) => setNoPatronymic(e.target.checked)}
              className="size-4 accent-accent"
            />
            Без отчества
          </label>
        </div>

        <PasswordField
          label="Пароль"
          autoComplete="new-password"
          placeholder="Минимум 6 символов"
          value={password}
          onChange={setPassword}
          hint={<span className={hintClass}>Минимум 6 символов, заглавная буква, цифра и спецсимвол</span>}
        />

        <PasswordField
          label="Подтверждение пароля"
          className="mb-6"
          autoComplete="new-password"
          placeholder="Повторите пароль"
          value={confirm}
          onChange={setConfirm}
          hint={mismatch && <span className="mt-1.5 block text-xs font-semibold text-err">Пароли не совпадают</span>}
        />

        {error && <Notice kind="err">{error}</Notice>}

        <motion.button type="submit" disabled={busy} whileTap={{ scale: 0.98 }} className={submitClass}>
          {busy ? 'Регистрируем…' : 'Зарегистрироваться'}
        </motion.button>
      </form>
    </AuthShell>
  )
}
