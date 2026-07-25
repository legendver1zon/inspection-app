import { useQueryClient } from '@tanstack/react-query'
import { api, type User } from '../lib/api'
import { C } from '../lib/palette'

export default function Header({ user }: { user: User }) {
  const queryClient = useQueryClient()

  async function logout() {
    await api.logout()
    queryClient.clear()
    window.location.href = '/login'
  }

  return (
    <header className="border-b" style={{ background: C.surface, borderColor: C.line }}>
      <div className="mx-auto flex h-14 max-w-5xl items-center gap-4 px-5">
        <a href="/inspections" className="flex items-center gap-2.5 font-extrabold" style={{ color: C.ink }}>
          <span className="grid size-8 place-items-center rounded-lg text-sm font-black text-white" style={{ background: C.accent }}>
            А
          </span>
          АктОсмотр
        </a>
        <div className="flex-1" />
        <span className="hidden text-sm font-semibold sm:block" style={{ color: C.muted }}>{user.initials}</span>
        <button onClick={logout} className="cursor-pointer text-sm font-semibold hover:underline" style={{ color: C.faint }}>
          Выйти
        </button>
      </div>
    </header>
  )
}
