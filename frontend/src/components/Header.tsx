import { useEffect, useState, type ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { api, type User } from '../lib/api'
import { C } from '../lib/palette'
import { isActive, uploadQueue, useUploadQueue } from '../lib/uploadQueue'

type NavItem = { to: string; label: string; icon: ReactNode }

export default function Header({ user }: { user: User }) {
  const queryClient = useQueryClient()
  const { pathname } = useLocation()
  const [open, setOpen] = useState(false)
  const queue = useUploadQueue()
  const active = queue.filter(isActive).length
  const failed = queue.filter((i) => i.status === 'failed').length

  useEffect(() => setOpen(false), [pathname])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open])

  async function logout() {
    await api.logout()
    queryClient.clear()
    window.location.href = '/login'
  }

  const links: NavItem[] = [
    { to: '/inspections', label: 'Осмотры', icon: <ListIcon /> },
    { to: '/dashboard', label: 'Статистика', icon: <ChartIcon /> },
  ]
  if (user.role === 'admin') links.push({ to: '/admin/users', label: 'Пользователи', icon: <UsersIcon /> })

  const isCurrent = (to: string) => pathname.startsWith(to)

  const avatar = user.avatar_url ? (
    <img src={user.avatar_url} alt="" className="size-9 rounded-full border object-cover" style={{ borderColor: C.line }} />
  ) : (
    <span className="grid size-9 place-items-center rounded-full text-[12px] font-extrabold text-white" style={{ background: C.accent }}>
      {(user.full_name[0] || '?').toUpperCase()}
    </span>
  )

  const queueBadge = (active > 0 || failed > 0) && (
    <button
      type="button"
      onClick={failed > 0 ? () => uploadQueue.retryAll() : undefined}
      title={failed > 0 ? 'Повторить отправку' : 'Фото отправляются'}
      className="rounded-full px-3 py-1.5 text-[12.5px] font-bold whitespace-nowrap"
      style={failed > 0 ? { background: C.errBg, color: C.err, cursor: 'pointer' } : { background: C.warnBg, color: C.warn }}
      aria-live="polite"
    >
      {failed > 0 ? `↻ ${failed}` : `↑ ${active}`}
    </button>
  )

  return (
    <header className="border-b" style={{ background: C.surface, borderColor: C.line }}>
      <div className="mx-auto flex h-14 max-w-5xl items-center gap-3 px-4 sm:gap-4 sm:px-5">
        <Link to="/inspections" className="flex items-center gap-2.5 font-extrabold" style={{ color: C.ink }}>
          <span className="grid size-8 place-items-center rounded-lg text-sm font-black text-white" style={{ background: C.accent }}>
            А
          </span>
          <span>АктОсмотр</span>
        </Link>

        <nav className="hidden gap-1 sm:flex" aria-label="Основная навигация">
          {links.map((l) => (
            <Link
              key={l.to}
              to={l.to}
              aria-current={isCurrent(l.to) ? 'page' : undefined}
              className="rounded-full px-3.5 py-2 text-[13.5px] font-bold whitespace-nowrap transition-colors"
              style={isCurrent(l.to) ? { background: C.accentSoft, color: C.accentDark } : { color: C.muted }}
            >
              {l.label}
            </Link>
          ))}
        </nav>

        <div className="flex-1" />
        {queueBadge}

        <Link to="/profile" className="hidden items-center gap-2.5 sm:flex" aria-label="Профиль">
          {avatar}
          <span className="text-sm font-semibold" style={{ color: C.muted }}>{user.initials}</span>
        </Link>
        <button onClick={logout} className="hidden cursor-pointer text-sm font-semibold hover:underline sm:block" style={{ color: C.faint }}>
          Выйти
        </button>

        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label="Открыть меню"
          aria-expanded={open}
          className="-mr-1 grid size-10 cursor-pointer place-items-center rounded-lg sm:hidden"
          style={{ color: C.ink }}
        >
          <MenuIcon />
        </button>
      </div>

      <AnimatePresence>
        {open && (
          <>
            <motion.div
              key="backdrop"
              className="fixed inset-0 z-40 sm:hidden"
              style={{ background: 'rgba(43,39,33,.4)' }}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setOpen(false)}
            />
            <motion.aside
              key="panel"
              role="dialog"
              aria-modal="true"
              aria-label="Меню"
              className="fixed inset-y-0 right-0 z-50 flex w-[280px] max-w-[85vw] flex-col shadow-[-12px_0_40px_rgba(43,39,33,.18)] sm:hidden"
              style={{ background: C.surface }}
              initial={{ x: '100%' }}
              animate={{ x: 0 }}
              exit={{ x: '100%' }}
              transition={{ type: 'spring', stiffness: 380, damping: 36 }}
            >
              <div className="flex items-center gap-3 border-b px-5 py-4" style={{ borderColor: C.line }}>
                {avatar}
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[14px] font-extrabold" style={{ color: C.ink }}>{user.full_name}</div>
                  <div className="truncate text-[12px]" style={{ color: C.muted }}>{user.email}</div>
                </div>
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  aria-label="Закрыть меню"
                  className="grid size-9 cursor-pointer place-items-center rounded-lg"
                  style={{ color: C.muted }}
                >
                  <CloseIcon />
                </button>
              </div>

              <nav className="flex flex-col gap-1 p-3" aria-label="Основная навигация">
                {links.map((l) => (
                  <DrawerLink key={l.to} to={l.to} current={isCurrent(l.to)} icon={l.icon}>
                    {l.label}
                  </DrawerLink>
                ))}
              </nav>

              <div className="mx-5 border-t" style={{ borderColor: C.line }} />

              <div className="flex flex-col gap-1 p-3">
                <DrawerLink to="/profile" current={isCurrent('/profile')} icon={<UserIcon />}>
                  Профиль
                </DrawerLink>
                <button
                  type="button"
                  onClick={logout}
                  className="flex cursor-pointer items-center gap-3 rounded-xl px-3.5 py-3 text-[15px] font-bold"
                  style={{ color: C.err }}
                >
                  <LogoutIcon />
                  Выйти
                </button>
              </div>
            </motion.aside>
          </>
        )}
      </AnimatePresence>
    </header>
  )
}

function DrawerLink({ to, current, icon, children }: { to: string; current: boolean; icon: ReactNode; children: ReactNode }) {
  return (
    <Link
      to={to}
      aria-current={current ? 'page' : undefined}
      className="flex items-center gap-3 rounded-xl px-3.5 py-3 text-[15px] font-bold transition-colors"
      style={current ? { background: C.accentSoft, color: C.accentDark } : { color: C.ink }}
    >
      <span style={{ color: current ? C.accentDark : C.muted }}>{icon}</span>
      {children}
    </Link>
  )
}

const iconProps = { width: 20, height: 20, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true }

function MenuIcon() {
  return (
    <svg {...iconProps} width={24} height={24}>
      <path d="M4 7h16M4 12h16M4 17h16" />
    </svg>
  )
}
function CloseIcon() {
  return (
    <svg {...iconProps}>
      <path d="m6 6 12 12M18 6 6 18" />
    </svg>
  )
}
function ListIcon() {
  return (
    <svg {...iconProps}>
      <path d="M8 6h13M8 12h13M8 18h13" />
      <circle cx="4" cy="6" r="1" fill="currentColor" />
      <circle cx="4" cy="12" r="1" fill="currentColor" />
      <circle cx="4" cy="18" r="1" fill="currentColor" />
    </svg>
  )
}
function ChartIcon() {
  return (
    <svg {...iconProps}>
      <path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
    </svg>
  )
}
function UsersIcon() {
  return (
    <svg {...iconProps}>
      <circle cx="9" cy="8" r="3.5" />
      <path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 7M21.5 20a6.5 6.5 0 0 0-4.5-6.2" />
    </svg>
  )
}
function UserIcon() {
  return (
    <svg {...iconProps}>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21a8 8 0 0 1 16 0" />
    </svg>
  )
}
function LogoutIcon() {
  return (
    <svg {...iconProps}>
      <path d="M10 4H5v16h5M14 8l4 4-4 4M18 12H9" />
    </svg>
  )
}
