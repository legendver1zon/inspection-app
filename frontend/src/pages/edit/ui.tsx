import { useEffect, type ButtonHTMLAttributes, type CSSProperties, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { C } from '../../lib/palette'
import { controlCls, controlStyle } from './controls'

/* Примитивы редактора в духе CRM: карточки с рамкой и радиусом 12,
   поля и кнопки 40 px на телефоне (36 на десктопе), чипы 20 px. */

export function Card({ title, extra, children, className = '' }: {
  title?: ReactNode
  extra?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`rounded-xl border ${className}`} style={{ background: C.surface, borderColor: C.line }}>
      {title && (
        <div className="flex min-h-12 items-center gap-3 border-b px-4 py-2" style={{ borderColor: C.line }}>
          <h2 className="min-w-0 flex-1 text-[14px] font-semibold" style={{ color: C.ink }}>{title}</h2>
          {extra}
        </div>
      )}
      <div className="p-3 sm:p-4">{children}</div>
    </section>
  )
}

export function Field({ label, hint, error, children, className = '' }: {
  label: ReactNode
  hint?: ReactNode
  error?: string | null
  children: ReactNode
  className?: string
}) {
  return (
    <label className={`flex min-w-0 flex-col gap-1 ${className}`}>
      <span className="text-[12px] font-medium" style={{ color: C.muted }}>{label}</span>
      {children}
      {error ? (
        <span className="text-[12px] font-medium" style={{ color: C.err }}>{error}</span>
      ) : hint ? (
        <span className="text-[12px]" style={{ color: C.faint }}>{hint}</span>
      ) : null}
    </label>
  )
}

export function TextInput({ invalid, className = '', ...rest }: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return <input className={`${controlCls} ${className}`} style={controlStyle(invalid)} {...rest} />
}

export function TextArea({ className = '', ...rest }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`${controlCls} h-auto py-2 ${className}`} style={controlStyle()} {...rest} />
}

export function Select({ className = '', children, ...rest }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={`${controlCls} ${className}`} style={controlStyle()} {...rest}>
      {children}
    </select>
  )
}

type Variant = 'primary' | 'default' | 'dashed' | 'text' | 'danger' | 'danger-text' | 'ghost-active'

export function Button({ variant = 'default', block, icon, className = '', children, style, ...rest }: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant
  block?: boolean
  icon?: ReactNode
}) {
  const base = 'inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-lg px-3.5 text-[14px] font-semibold whitespace-nowrap transition-colors disabled:cursor-default disabled:opacity-50 sm:h-9 sm:text-[13px]'
  const styles: Record<Variant, CSSProperties> = {
    primary: { background: C.accent, color: '#fff' },
    default: { background: C.surface, color: C.ink, border: `1px solid ${C.line}` },
    dashed: { background: 'transparent', color: C.accentDark, border: `1px dashed ${C.rail}` },
    text: { background: 'transparent', color: C.accentDark },
    danger: { background: C.surface, color: C.err, border: `1px solid ${C.err}` },
    'danger-text': { background: 'transparent', color: C.err },
    'ghost-active': { background: C.accentSoft, color: C.accentDark, border: `1px solid ${C.accent}` },
  }
  return (
    <button type="button" className={`${base} ${block ? 'w-full' : ''} ${className}`} style={{ ...styles[variant], ...style }} {...rest}>
      {icon}
      {children}
    </button>
  )
}

export function Chip({ variant = 'neutral', children }: { variant?: 'info' | 'neutral' | 'success' | 'warn' | 'danger'; children: ReactNode }) {
  const styles = {
    info: { background: C.accentSoft, color: C.accentDark },
    neutral: { background: C.track, color: C.muted },
    success: { background: C.okBg, color: C.ok },
    warn: { background: C.warnBg, color: C.warn },
    danger: { background: C.errBg, color: C.err },
  }[variant]
  return (
    <span className="inline-flex h-5 items-center rounded-[5px] px-1.5 text-[12px] font-semibold whitespace-nowrap" style={styles}>
      {children}
    </span>
  )
}

export function Collapse({ open, onToggle, header, children, className = '' }: {
  open: boolean
  onToggle: () => void
  header: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <div className={`overflow-hidden rounded-xl border ${className}`} style={{ background: C.surface, borderColor: C.line }}>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex min-h-12 w-full cursor-pointer items-center gap-3 px-4 py-2 text-left"
        style={{ color: C.ink }}
      >
        <ChevronIcon open={open} />
        <div className="min-w-0 flex-1">{header}</div>
      </button>
      {open && (
        <div className="border-t p-3 sm:p-4" style={{ borderColor: C.line }}>
          {children}
        </div>
      )}
    </div>
  )
}

export function Drawer({ open, onClose, title, footer, children }: {
  open: boolean
  onClose: () => void
  title: ReactNode
  footer?: ReactNode
  children: ReactNode
}) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            key="backdrop"
            className="fixed inset-0 z-40"
            style={{ background: 'rgba(43,39,33,.45)' }}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={onClose}
          />
          <motion.div
            key="panel"
            role="dialog"
            aria-modal="true"
            className="fixed inset-x-0 bottom-0 z-50 flex h-[90dvh] flex-col rounded-t-2xl sm:inset-y-0 sm:right-0 sm:left-auto sm:h-auto sm:w-[480px] sm:rounded-none"
            style={{ background: C.surface, boxShadow: '0 -12px 40px rgba(43,39,33,.18)' }}
            initial={{ y: '100%' }}
            animate={{ y: 0 }}
            exit={{ y: '100%' }}
            transition={{ type: 'spring', stiffness: 380, damping: 38 }}
          >
            <div className="flex min-h-14 items-center gap-2 border-b px-4" style={{ borderColor: C.line }}>
              <button type="button" onClick={onClose} aria-label="Закрыть" className="-ml-1 grid size-9 cursor-pointer place-items-center rounded-lg" style={{ color: C.muted }}>
                <CloseIcon />
              </button>
              <div className="min-w-0 flex-1 text-[15px] font-semibold" style={{ color: C.ink }}>{title}</div>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">{children}</div>
            {footer && (
              <div className="flex flex-col gap-2 border-t px-4 pt-3 pb-[max(12px,env(safe-area-inset-bottom))]" style={{ borderColor: C.line }}>
                {footer}
              </div>
            )}
          </motion.div>
        </>
      )}
    </AnimatePresence>
  )
}

/* Иконки */
const ip = { width: 18, height: 18, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true }

export function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg {...ip} width={16} height={16} style={{ color: C.muted, transform: open ? 'rotate(90deg)' : 'none', transition: 'transform .15s' }}>
      <path d="m9 6 6 6-6 6" />
    </svg>
  )
}
export function ChevronRightIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="m9 6 6 6-6 6" />
    </svg>
  )
}
export function PlusIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="M12 5v14M5 12h14" />
    </svg>
  )
}
export function TrashIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3" />
    </svg>
  )
}
export function CheckIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="m5 12 5 5L20 7" />
    </svg>
  )
}
export function CameraIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="M4 8h3l2-3h6l2 3h3v11H4z" />
      <circle cx="12" cy="13" r="3.5" />
    </svg>
  )
}
export function ArrowLeftIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <path d="M19 12H5M11 6l-6 6 6 6" />
    </svg>
  )
}
export function SearchIcon() {
  return (
    <svg {...ip} width={16} height={16}>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </svg>
  )
}
export function CloseIcon() {
  return (
    <svg {...ip}>
      <path d="m6 6 12 12M18 6 6 18" />
    </svg>
  )
}

export function Switch({ checked, onChange, label, hint }: {
  checked: boolean
  onChange: (v: boolean) => void
  label: ReactNode
  hint?: ReactNode
}) {
  return (
    <button type="button" role="switch" aria-checked={checked} onClick={() => onChange(!checked)} className="flex w-full cursor-pointer items-center gap-3 text-left">
      <span className="relative inline-flex h-6 w-11 flex-none rounded-full transition-colors" style={{ background: checked ? C.accent : C.track }}>
        <span className="absolute top-0.5 size-5 rounded-full bg-white shadow transition-transform" style={{ transform: checked ? 'translateX(22px)' : 'translateX(2px)' }} />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="text-[14px] font-medium">{label}</span>
        {hint && <span className="text-[12px]" style={{ color: C.faint }}>{hint}</span>}
      </span>
    </button>
  )
}
