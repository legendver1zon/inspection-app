import { useState, type ReactNode } from 'react'
import { motion } from 'framer-motion'
import EyeIcon from './EyeIcon'

export const fieldClass =
  'w-full rounded-xl border border-line bg-surface px-4 py-3 text-[15px] text-ink placeholder-faint transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(62,104,168,.15)] focus:outline-none'
export const labelClass = 'mb-1.5 block text-xs font-bold tracking-wide text-muted uppercase'
export const hintClass = 'mt-1.5 block text-xs text-muted'
export const cardClass =
  'rounded-3xl border border-line bg-surface p-6 shadow-[0_1px_2px_rgba(43,39,33,.05),0_16px_40px_rgba(43,39,33,.08)] sm:p-8'
export const submitClass =
  'w-full cursor-pointer rounded-xl bg-accent py-3.5 text-[15px] font-extrabold text-on-accent shadow-[0_5px_16px_rgba(62,104,168,.28)] transition-colors hover:bg-accent-hover disabled:opacity-60'
export const linkClass = 'font-semibold text-accent hover:underline'

export function AuthShell({
  subtitle,
  footer,
  children,
}: {
  subtitle: string
  footer: ReactNode
  children: ReactNode
}) {
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
          <p className="text-center text-sm text-muted">{subtitle}</p>
        </div>

        {children}

        <p className="mt-6 text-center text-sm text-muted">{footer}</p>
      </motion.div>
    </motion.main>
  )
}

export function Notice({ kind, children }: { kind: 'err' | 'ok'; children: ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, height: 0 }}
      animate={{ opacity: 1, height: 'auto' }}
      role={kind === 'err' ? 'alert' : 'status'}
      className={`mb-4 overflow-hidden rounded-xl px-4 py-3 text-sm font-semibold whitespace-pre-line ${
        kind === 'err' ? 'bg-err-bg text-err' : 'bg-ok-bg text-ok'
      }`}
    >
      {children}
    </motion.div>
  )
}

export function PasswordField({
  label,
  value,
  onChange,
  autoComplete,
  placeholder = '••••••••',
  className = 'mb-4',
  hint,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  autoComplete: 'current-password' | 'new-password'
  placeholder?: string
  className?: string
  hint?: ReactNode
}) {
  const [show, setShow] = useState(false)
  return (
    <label className={`block ${className}`}>
      <span className={labelClass}>{label}</span>
      <div className="relative">
        <input
          type={show ? 'text' : 'password'}
          required
          autoComplete={autoComplete}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className={`${fieldClass} pr-12`}
        />
        <button
          type="button"
          onClick={() => setShow((v) => !v)}
          aria-label={show ? 'Скрыть пароль' : 'Показать пароль'}
          className="absolute top-1/2 right-3 -translate-y-1/2 cursor-pointer p-1 text-faint transition-colors hover:text-muted"
        >
          <EyeIcon crossed={show} />
        </button>
      </div>
      {hint}
    </label>
  )
}
