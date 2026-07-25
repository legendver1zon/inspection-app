import { useDeferredValue, useState } from 'react'
import { motion } from 'framer-motion'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useQueryClient } from '@tanstack/react-query'
import { api, type ActCard, type User } from '../lib/api'

/* Концепт V1 «Линия»: рабочий инструмент в духе Linear/Notion.
   Сайдбар, плотные строки, холодные нейтральные, индиго-акцент.
   Самодостаточная палитра — не зависит от глобальной темы. */

const C = {
  bg: '#F7F8FA',
  side: '#FFFFFF',
  surface: '#FFFFFF',
  line: '#E6E8EC',
  ink: '#101828',
  muted: '#5D6675',
  faint: '#98A1B0',
  accent: '#4F46E5',
  accentSoft: '#EEF0FE',
  ok: '#067647',
  okBg: '#E8F5EE',
  warn: '#B54708',
  warnBg: '#FDF2E3',
  err: '#B42318',
  errBg: '#FBEAE9',
}

export default function V1Inspections({ user }: { user: User }) {
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const deferredQ = useDeferredValue(q)
  const queryClient = useQueryClient()

  const { data, isLoading } = useQuery({
    queryKey: ['inspections', deferredQ, page],
    queryFn: () => api.inspections({ q: deferredQ, page }),
    placeholderData: keepPreviousData,
  })

  async function logout() {
    await api.logout()
    queryClient.clear()
    window.location.href = '/login'
  }

  return (
    <div className="flex min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      {/* ===== Сайдбар ===== */}
      <aside
        className="fixed inset-y-0 left-0 z-20 hidden w-60 flex-col border-r px-3 py-4 md:flex"
        style={{ background: C.side, borderColor: C.line }}
      >
        <div className="mb-6 flex items-center gap-2.5 px-2">
          <span
            className="grid size-7 place-items-center rounded-lg text-[13px] font-black text-white"
            style={{ background: C.accent }}
          >
            А
          </span>
          <span className="text-[15px] font-bold">АктОсмотр</span>
        </div>

        <nav className="grid gap-0.5 text-[13.5px] font-semibold">
          <a
            href="/inspections"
            className="flex items-center gap-2.5 rounded-lg px-2.5 py-2"
            style={{ background: C.accentSoft, color: C.accent }}
          >
            <DocIcon /> Осмотры
            <span className="ml-auto rounded-md px-1.5 text-xs font-bold" style={{ background: '#fff', color: C.muted }}>
              {(data?.draft_count ?? 0) + (data?.completed_count ?? 0) || ''}
            </span>
          </a>
          <span className="flex cursor-not-allowed items-center gap-2.5 rounded-lg px-2.5 py-2" style={{ color: C.faint }}>
            <ChartIcon /> Статистика
          </span>
          {user.role === 'admin' && (
            <span className="flex cursor-not-allowed items-center gap-2.5 rounded-lg px-2.5 py-2" style={{ color: C.faint }}>
              <UsersIcon /> Пользователи
            </span>
          )}
        </nav>

        <div className="mt-auto border-t pt-3" style={{ borderColor: C.line }}>
          <div className="flex items-center gap-2.5 px-2">
            <span
              className="grid size-8 place-items-center rounded-full text-xs font-extrabold text-white"
              style={{ background: C.accent }}
            >
              {(user.full_name[0] || '?').toUpperCase()}
            </span>
            <div className="min-w-0 flex-1">
              <div className="truncate text-[13px] font-bold">{user.full_name}</div>
              <button onClick={logout} className="cursor-pointer text-xs hover:underline" style={{ color: C.muted }}>
                Выйти
              </button>
            </div>
          </div>
        </div>
      </aside>

      {/* ===== Контент ===== */}
      <div className="min-w-0 flex-1 md:pl-60">
        {/* Тулбар */}
        <div
          className="sticky top-0 z-10 flex items-center gap-3 border-b px-5 py-3 backdrop-blur-md"
          style={{ background: 'rgba(247,248,250,.88)', borderColor: C.line }}
        >
          <div className="relative max-w-xl flex-1">
            <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2" style={{ color: C.faint }}>
              <SearchIcon />
            </span>
            <input
              type="search"
              value={q}
              onChange={(e) => { setQ(e.target.value); setPage(1) }}
              placeholder="Поиск по актам…"
              className="w-full rounded-lg border py-2 pr-14 pl-9 text-sm outline-none focus:ring-2"
              style={{ background: C.surface, borderColor: C.line, color: C.ink, ['--tw-ring-color' as string]: C.accentSoft }}
            />
            <kbd
              className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 rounded border px-1.5 py-0.5 text-[10.5px] font-bold"
              style={{ borderColor: C.line, color: C.faint }}
            >
              ⌘K
            </kbd>
          </div>
          <div className="flex-1" />
          <a
            href="/inspections/new"
            className="rounded-lg px-3.5 py-2 text-[13.5px] font-bold text-white transition-opacity hover:opacity-90"
            style={{ background: C.accent }}
          >
            Новый осмотр
          </a>
        </div>

        <main className="mx-auto max-w-4xl px-5 py-6 pb-24">
          <h1 className="mb-5 text-xl font-bold">Осмотры</h1>

          {/* В работе */}
          <SectionLabel text="В работе" count={data?.draft_count} color={C.warn} />
          <div className="mb-7 overflow-hidden rounded-xl border" style={{ background: C.surface, borderColor: C.line }}>
            {isLoading ? (
              <RowSkeletons n={2} />
            ) : data && data.drafts.length > 0 ? (
              data.drafts.map((act, i) => <DraftRow key={act.id} act={act} first={i === 0} />)
            ) : (
              <EmptyRow text={deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Нет актов в работе'} />
            )}
          </div>

          {/* Завершённые */}
          <SectionLabel text="Завершённые" count={data?.completed_count} color={C.ok} />
          <div className="overflow-hidden rounded-xl border" style={{ background: C.surface, borderColor: C.line }}>
            {isLoading ? (
              <RowSkeletons n={3} />
            ) : data && data.completed.length > 0 ? (
              <table className="w-full text-[13.5px]">
                <tbody>
                  {data.completed.map((act) => (
                    <tr key={act.id} className="group border-t first:border-t-0 hover:bg-[#FAFBFC]" style={{ borderColor: C.line }}>
                      <td className="py-2.5 pl-4 font-mono text-[13px] font-bold whitespace-nowrap tnum">
                        <a href={`/inspections/${act.id}`} style={{ color: C.ink }}>{act.act_number}</a>
                      </td>
                      <td className="px-3 py-2.5" style={{ color: C.ink }}>
                        <span className="line-clamp-1">{act.address}</span>
                      </td>
                      <td className="hidden px-3 py-2.5 whitespace-nowrap lg:table-cell" style={{ color: C.muted }}>
                        {act.owner_name}
                      </td>
                      <td className="px-3 py-2.5 text-right whitespace-nowrap tnum" style={{ color: C.muted }}>
                        {act.defects} деф · {act.photos} фото
                      </td>
                      <td className="px-3 py-2.5 whitespace-nowrap" style={{ color: C.faint }}>{act.date}</td>
                      <td className="py-2.5 pr-4 text-right">
                        <span className="rounded-md px-2 py-1 text-[11.5px] font-bold" style={{ background: C.okBg, color: C.ok }}>
                          Завершён
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <EmptyRow text={deferredQ ? 'Не найдено' : 'Завершённых пока нет'} />
            )}
            {data && data.total_pages > 1 && (
              <div className="flex items-center justify-between border-t px-4 py-2.5 text-xs" style={{ borderColor: C.line, color: C.muted }}>
                <span>Стр. {page} из {data.total_pages}</span>
                <div className="flex gap-1">
                  <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}
                          className="cursor-pointer rounded border px-2 py-1 disabled:opacity-40" style={{ borderColor: C.line }}>‹</button>
                  <button disabled={page >= data.total_pages} onClick={() => setPage((p) => p + 1)}
                          className="cursor-pointer rounded border px-2 py-1 disabled:opacity-40" style={{ borderColor: C.line }}>›</button>
                </div>
              </div>
            )}
          </div>
        </main>
      </div>
    </div>
  )
}

/* ===== Строка черновика ===== */

function DraftRow({ act, first }: { act: ActCard; first: boolean }) {
  return (
    <motion.div
      initial={{ opacity: 0, x: -8 }}
      animate={{ opacity: 1, x: 0 }}
      className={`flex items-center gap-4 px-4 py-3 hover:bg-[#FAFBFC] ${first ? '' : 'border-t'}`}
      style={{ borderColor: C.line }}
    >
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2.5">
          <a href={`/inspections/${act.id}`} className="font-mono text-[13.5px] font-bold hover:underline tnum" style={{ color: C.ink }}>
            {act.act_number}
          </a>
          <span className="truncate text-[13.5px] font-medium" style={{ color: C.ink }}>{act.address || 'Адрес не указан'}</span>
        </div>
        <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: C.muted }}>
          <span>{act.owner_name || 'Собственник не указан'}</span>
          <Dot />
          <span className="tnum">{act.defects} деф · {act.photos} фото</span>
          {act.cloud_state === 'queue' && (<><Dot /><span className="font-semibold" style={{ color: C.warn }}>{act.cloud_n} в очереди</span></>)}
          {act.cloud_state === 'error' && (<><Dot /><span className="font-semibold" style={{ color: C.err }}>сбой выгрузки</span></>)}
        </div>
      </div>

      {act.rooms > 0 && (
        <div className="hidden w-36 shrink-0 sm:block">
          <div className="mb-1 flex justify-between text-[11px]" style={{ color: C.muted }}>
            <span className="tnum">{act.filled}/{act.rooms} пом.</span>
            <span className="tnum">{act.percent}%</span>
          </div>
          <div className="h-1 overflow-hidden rounded-full" style={{ background: C.line }}>
            <motion.i
              initial={{ width: 0 }}
              animate={{ width: `${act.percent}%` }}
              transition={{ type: 'spring', stiffness: 90, damping: 20 }}
              className="block h-full rounded-full"
              style={{ background: C.accent }}
            />
          </div>
        </div>
      )}

      <a
        href={`/inspections/${act.id}/edit`}
        className="shrink-0 rounded-lg px-3 py-1.5 text-[12.5px] font-bold transition-colors"
        style={{ background: C.accentSoft, color: C.accent }}
      >
        Продолжить
      </a>
    </motion.div>
  )
}

/* ===== Мелочи ===== */

function SectionLabel({ text, count, color }: { text: string; count?: number; color: string }) {
  return (
    <div className="mb-2 flex items-center gap-2 px-1">
      <span className="size-1.5 rounded-full" style={{ background: color }} />
      <span className="text-[11.5px] font-bold tracking-wider uppercase" style={{ color: C.muted }}>{text}</span>
      <span className="text-[11.5px] font-bold tnum" style={{ color: C.faint }}>{count ?? '…'}</span>
    </div>
  )
}
function Dot() { return <span className="size-0.5 rounded-full" style={{ background: C.faint }} /> }
function EmptyRow({ text }: { text: string }) {
  return <div className="px-4 py-8 text-center text-sm" style={{ color: C.muted }}>{text}</div>
}
function RowSkeletons({ n }: { n: number }) {
  return (
    <div className="grid gap-px">
      {Array.from({ length: n }, (_, i) => (
        <div key={i} className="h-14 animate-pulse motion-reduce:animate-none" style={{ background: '#F1F3F5' }} />
      ))}
    </div>
  )
}

function SearchIcon() {
  return <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
}
function DocIcon() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2" /><rect x="9" y="3" width="6" height="4" rx="1" /><path d="M9 12h6M9 16h4" /></svg>
}
function ChartIcon() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M18 20V10M12 20V4M6 20v-6" /></svg>
}
function UsersIcon() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></svg>
}
