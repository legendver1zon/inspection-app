import { useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { useQueryClient } from '@tanstack/react-query'
import { api, type User } from '../lib/api'
import { useTheme } from '../lib/theme'

export default function TopBar({ user }: { user: User }) {
  const { theme, toggle } = useTheme()
  const [menuOpen, setMenuOpen] = useState(false)
  const queryClient = useQueryClient()

  async function logout() {
    await api.logout()
    queryClient.setQueryData(['me'], undefined)
    queryClient.clear()
    window.location.href = '/login'
  }

  return (
    <header className="sticky top-0 z-30 border-b border-line-soft bg-surface/85 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-5 px-4 sm:px-6">
        <a href="/inspections" className="flex items-center gap-2.5 text-[15.5px] font-extrabold text-ink">
          <motion.span
            whileHover={{ rotate: -6, scale: 1.06 }}
            transition={{ type: 'spring', stiffness: 400, damping: 15 }}
            className="grid size-8 place-items-center rounded-[9px] bg-gradient-to-br from-accent-bright to-accent font-black text-white shadow-[0_2px_10px_rgba(233,120,23,.35)]"
          >
            А
          </motion.span>
          АктОсмотр
        </a>

        <nav className="hidden gap-0.5 sm:flex" aria-label="Основная навигация">
          <a href="/inspections" aria-current="page"
             className="rounded-lg bg-inset px-3.5 py-2 text-sm font-semibold text-ink">
            Осмотры
          </a>
          <span className="cursor-not-allowed rounded-lg px-3.5 py-2 text-sm font-semibold text-faint" title="Скоро">
            Статистика
          </span>
          {user.role === 'admin' && (
            <span className="cursor-not-allowed rounded-lg px-3.5 py-2 text-sm font-semibold text-faint" title="Скоро">
              Пользователи
            </span>
          )}
        </nav>

        <div className="flex-1" />

        <button
          onClick={toggle}
          aria-label="Переключить тему"
          className="grid size-10 cursor-pointer place-items-center rounded-[10px] border border-line bg-surface text-muted transition-colors hover:border-muted hover:text-ink"
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.span
              key={theme}
              initial={{ rotate: -90, opacity: 0 }}
              animate={{ rotate: 0, opacity: 1 }}
              exit={{ rotate: 90, opacity: 0 }}
              transition={{ duration: 0.18 }}
              className="grid place-items-center"
            >
              {theme === 'dark' ? <SunIcon /> : <MoonIcon />}
            </motion.span>
          </AnimatePresence>
        </button>

        <div className="relative">
          <button
            onClick={() => setMenuOpen((v) => !v)}
            aria-label="Меню пользователя"
            aria-expanded={menuOpen}
            className="grid size-9 cursor-pointer place-items-center overflow-hidden rounded-full bg-gradient-to-br from-accent-bright to-accent text-[13px] font-extrabold text-white shadow-[0_2px_8px_rgba(233,120,23,.35)] transition-transform hover:scale-105"
          >
            {user.avatar_url ? (
              <img src={user.avatar_url} alt="" className="size-full object-cover" />
            ) : (
              shortInitials(user.initials || user.full_name)
            )}
          </button>

          <AnimatePresence>
            {menuOpen && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setMenuOpen(false)} />
                <motion.div
                  initial={{ opacity: 0, y: -6, scale: 0.97 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -6, scale: 0.97 }}
                  transition={{ duration: 0.15 }}
                  className="absolute right-0 z-50 mt-2 w-64 overflow-hidden rounded-2xl border border-line bg-surface shadow-xl"
                >
                  <div className="border-b border-line-soft px-4 py-3">
                    <div className="truncate text-sm font-bold text-ink">{user.full_name}</div>
                    <div className="truncate text-xs text-muted">{user.email}</div>
                  </div>
                  <button
                    onClick={logout}
                    className="w-full cursor-pointer px-4 py-3 text-left text-sm font-semibold text-err transition-colors hover:bg-err-bg"
                  >
                    Выйти
                  </button>
                </motion.div>
              </>
            )}
          </AnimatePresence>
        </div>
      </div>
    </header>
  )
}

function shortInitials(s: string) {
  const parts = s.split(/\s+/).filter(Boolean)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return (s.slice(0, 2) || '?').toUpperCase()
}

function MoonIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8Z" />
    </svg>
  )
}

function SunIcon() {
  return (
    <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4 1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </svg>
  )
}
