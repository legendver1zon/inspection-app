import { Link, useLocation } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api, type User } from '../lib/api'
import { C } from '../lib/palette'
import { isActive, uploadQueue, useUploadQueue } from '../lib/uploadQueue'

export default function Header({ user }: { user: User }) {
  const queryClient = useQueryClient()
  const { pathname } = useLocation()
  const queue = useUploadQueue()
  const active = queue.filter(isActive).length
  const failed = queue.filter((i) => i.status === 'failed').length

  async function logout() {
    await api.logout()
    queryClient.clear()
    window.location.href = '/login'
  }

  const links: [string, string][] = [
    ['/inspections', 'Осмотры'],
    ['/dashboard', 'Статистика'],
  ]
  if (user.role === 'admin') links.push(['/admin/users', 'Пользователи'])

  return (
    <header className="border-b" style={{ background: C.surface, borderColor: C.line }}>
      <div className="mx-auto flex h-14 max-w-5xl items-center gap-4 px-5">
        <Link to="/inspections" className="flex items-center gap-2.5 font-extrabold" style={{ color: C.ink }}>
          <span className="grid size-8 place-items-center rounded-lg text-sm font-black text-white" style={{ background: C.accent }}>
            А
          </span>
          <span className="hidden sm:block">АктОсмотр</span>
        </Link>

        <nav className="flex gap-1" aria-label="Основная навигация">
          {links.map(([to, label]) => {
            const active = pathname.startsWith(to)
            return (
              <Link
                key={to}
                to={to}
                aria-current={active ? 'page' : undefined}
                className="rounded-full px-3.5 py-2 text-[13.5px] font-bold transition-colors"
                style={active ? { background: C.accentSoft, color: C.accentDark } : { color: C.muted }}
              >
                {label}
              </Link>
            )
          })}
        </nav>

        <div className="flex-1" />
        {(active > 0 || failed > 0) && (
          <button
            type="button"
            onClick={failed > 0 ? () => uploadQueue.retryAll() : undefined}
            title={failed > 0 ? 'Повторить отправку' : 'Фото отправляются'}
            className="rounded-full px-3 py-1.5 text-[12.5px] font-bold"
            style={failed > 0 ? { background: C.errBg, color: C.err, cursor: 'pointer' } : { background: C.warnBg, color: C.warn }}
            aria-live="polite"
          >
            {failed > 0 ? `↻ не отправлено: ${failed}` : `↑ фото: ${active}`}
          </button>
        )}
        <Link to="/profile" className="flex items-center gap-2.5" aria-label="Профиль">
          {user.avatar_url ? (
            <img src={user.avatar_url} alt="" className="size-8 rounded-full border object-cover" style={{ borderColor: C.line }} />
          ) : (
            <span className="grid size-8 place-items-center rounded-full text-[11px] font-extrabold text-white" style={{ background: C.accent }}>
              {(user.full_name[0] || '?').toUpperCase()}
            </span>
          )}
          <span className="hidden text-sm font-semibold sm:block" style={{ color: C.muted }}>{user.initials}</span>
        </Link>
        <button onClick={logout} className="cursor-pointer text-sm font-semibold hover:underline" style={{ color: C.faint }}>
          Выйти
        </button>
      </div>
    </header>
  )
}
