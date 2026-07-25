import { useDeferredValue, useState } from 'react'
import { motion } from 'framer-motion'
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type ActCard, type User } from '../lib/api'

/* Концепт V3 «Студия»: тёмный премиум — стекло, свечения, градиентный
   акцент оранж→розовый. Сознательно только тёмный: это его мир. */

export default function V3Inspections({ user }: { user: User }) {
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
    <div className="min-h-dvh text-[#F2EFEA]" style={{ background: '#0C0E13' }}>
      {/* Фоновые свечения */}
      <div aria-hidden="true" className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="absolute -top-40 left-1/4 size-[560px] rounded-full opacity-25 blur-[130px]" style={{ background: '#FF7A1A' }} />
        <div className="absolute top-1/3 -right-40 size-[480px] rounded-full opacity-15 blur-[130px]" style={{ background: '#E33D6F' }} />
      </div>

      {/* Топбар-стекло */}
      <header className="sticky top-0 z-30 border-b border-white/8 bg-[#0C0E13]/70 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-5">
          <a href="/inspections" className="flex items-center gap-3 font-extrabold">
            <span className="grid size-9 place-items-center rounded-xl bg-gradient-to-br from-[#FF9838] via-[#FF7A1A] to-[#E33D6F] text-[15px] font-black text-white shadow-[0_0_24px_rgba(255,122,26,.5)]">
              А
            </span>
            <span className="text-[16px] tracking-tight">АктОсмотр</span>
          </a>
          <nav className="hidden gap-1 text-sm font-semibold text-white/55 md:flex">
            <span className="rounded-full bg-white/10 px-4 py-2 text-white">Осмотры</span>
            <span className="cursor-not-allowed rounded-full px-4 py-2">Статистика</span>
            {user.role === 'admin' && <span className="cursor-not-allowed rounded-full px-4 py-2">Пользователи</span>}
          </nav>
          <div className="flex-1" />
          <button onClick={logout} className="cursor-pointer text-sm font-semibold text-white/45 transition-colors hover:text-white">
            Выйти
          </button>
          <span className="grid size-9 place-items-center rounded-full border border-white/15 bg-white/8 text-xs font-extrabold text-[#FFB273]">
            {(user.full_name[0] || '?').toUpperCase()}
          </span>
        </div>
      </header>

      <main className="relative mx-auto max-w-6xl px-5 pt-10 pb-28">
        <motion.div initial={{ opacity: 0, y: 14 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}>
          <div className="mb-8 flex flex-wrap items-end gap-5">
            <div>
              <h1 className="text-[34px] font-extrabold tracking-tight">
                Осмотры<span className="bg-gradient-to-r from-[#FF9838] to-[#E33D6F] bg-clip-text text-transparent">.</span>
              </h1>
              <p className="mt-1 text-sm text-white/45">
                {data ? <>{data.draft_count} в работе · {data.completed_count} завершено</> : '…'}
              </p>
            </div>
            <div className="flex-1" />
            <motion.a
              href="/inspections/new"
              whileHover={{ scale: 1.03 }}
              whileTap={{ scale: 0.97 }}
              className="rounded-2xl bg-gradient-to-r from-[#FF8A2A] to-[#E33D6F] px-6 py-3.5 text-[15px] font-extrabold text-white shadow-[0_8px_30px_rgba(255,122,26,.35)]"
            >
              ＋ Новый осмотр
            </motion.a>
          </div>

          <div className="relative mb-10 max-w-2xl">
            <span className="pointer-events-none absolute top-1/2 left-4.5 -translate-y-1/2 text-white/35">
              <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
            </span>
            <input
              type="search"
              value={q}
              onChange={(e) => { setQ(e.target.value); setPage(1) }}
              placeholder="Найти акт: номер, адрес, фамилия"
              className="w-full rounded-2xl border border-white/10 bg-white/6 py-4 pr-5 pl-12 text-[15px] text-white backdrop-blur-md transition-all outline-none placeholder:text-white/30 focus:border-[#FF8A2A]/60 focus:bg-white/8 focus:shadow-[0_0_0_4px_rgba(255,138,42,.12)]"
            />
          </div>
        </motion.div>

        {/* В работе */}
        <GroupTitle icon="⚡" title="В работе" count={data?.draft_count} />
        {isLoading ? (
          <div className="mb-12 grid gap-5 md:grid-cols-2"><GlassSkeleton /><GlassSkeleton /></div>
        ) : data && data.drafts.length > 0 ? (
          <motion.div
            variants={{ show: { transition: { staggerChildren: 0.07 } } }}
            initial="hidden"
            animate="show"
            className="mb-12 grid gap-5 md:grid-cols-2"
          >
            {data.drafts.map((act) => <GlassCard key={act.id} act={act} />)}
          </motion.div>
        ) : (
          <p className="mb-12 py-6 text-center text-white/40">
            {deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Нет актов в работе'}
          </p>
        )}

        {/* Завершённые */}
        <GroupTitle icon="✓" title="Завершённые" count={data?.completed_count} />
        {isLoading ? (
          <GlassSkeleton />
        ) : data && data.completed.length > 0 ? (
          <div className="grid gap-3">
            {data.completed.map((act, i) => (
              <motion.a
                key={act.id}
                href={`/inspections/${act.id}`}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05 }}
                className="group flex items-center gap-5 rounded-2xl border border-white/8 bg-white/4 px-5 py-4 backdrop-blur-md transition-colors hover:border-white/20 hover:bg-white/7"
              >
                <span className="font-mono text-[15px] font-bold text-white/90 tnum">{act.act_number}</span>
                <span className="min-w-0 flex-1 truncate text-[14.5px] text-white/70">{act.address}</span>
                <span className="hidden text-[13px] whitespace-nowrap text-white/40 lg:block">{act.owner_name}</span>
                <span className="hidden text-[13px] whitespace-nowrap text-white/40 tnum sm:block">
                  {act.defects} деф · {act.photos} фото
                </span>
                <span className="inline-flex items-center gap-1.5 rounded-full border border-[#5DD397]/30 bg-[#5DD397]/10 px-3 py-1.5 text-xs font-bold whitespace-nowrap text-[#5DD397]">
                  ✓ Завершён
                </span>
                <span className="text-white/25 transition-transform group-hover:translate-x-1">→</span>
              </motion.a>
            ))}
            {data.total_pages > 1 && (
              <div className="mt-2 flex items-center justify-center gap-3 text-sm text-white/45">
                <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}
                        className="cursor-pointer rounded-full border border-white/12 px-4 py-2 font-bold disabled:opacity-30">‹</button>
                <span className="tnum">{page} / {data.total_pages}</span>
                <button disabled={page >= data.total_pages} onClick={() => setPage((p) => p + 1)}
                        className="cursor-pointer rounded-full border border-white/12 px-4 py-2 font-bold disabled:opacity-30">›</button>
              </div>
            )}
          </div>
        ) : (
          <p className="py-6 text-center text-white/40">{deferredQ ? 'Не найдено' : 'Завершённых пока нет'}</p>
        )}
      </main>
    </div>
  )
}

function GroupTitle({ icon, title, count }: { icon: string; title: string; count?: number }) {
  return (
    <div className="mb-5 flex items-center gap-3">
      <span className="grid size-7 place-items-center rounded-lg border border-white/10 bg-white/6 text-[13px]">{icon}</span>
      <h2 className="text-[13px] font-extrabold tracking-[.18em] text-white/60 uppercase">{title}</h2>
      <span className="rounded-full border border-white/10 bg-white/6 px-2.5 py-0.5 text-xs font-bold text-white/60 tnum">{count ?? '…'}</span>
      <span className="h-px flex-1 bg-gradient-to-r from-white/15 to-transparent" />
    </div>
  )
}

function GlassCard({ act }: { act: ActCard }) {
  return (
    <motion.article
      variants={{ hidden: { opacity: 0, y: 18 }, show: { opacity: 1, y: 0 } }}
      whileHover={{ y: -3 }}
      transition={{ type: 'spring', stiffness: 300, damping: 24 }}
      className="group relative overflow-hidden rounded-3xl border border-white/10 bg-white/5 p-6 backdrop-blur-md"
    >
      {/* Свечение при наведении */}
      <div aria-hidden="true" className="pointer-events-none absolute -top-24 -right-24 size-56 rounded-full bg-[#FF7A1A] opacity-0 blur-[90px] transition-opacity duration-500 group-hover:opacity-20" />

      <div className="mb-3 flex items-baseline gap-3">
        <a href={`/inspections/${act.id}`} className="font-mono text-[21px] font-extrabold tracking-tight text-white transition-colors tnum hover:text-[#FFB273]">
          № {act.act_number}
        </a>
        <span className="ml-auto text-[13px] text-white/35">{act.date}</span>
      </div>

      <div className="text-[16px] font-semibold text-white/90">{act.address || 'Адрес не указан'}</div>
      <div className="mt-1 mb-5 text-[13.5px] text-white/45">
        {act.owner_name || 'Собственник не указан'}
        {act.rooms > 0 && <> · {act.rooms} пом.</>}
        {act.total_area > 0 && <> · {act.total_area} м²</>}
      </div>

      {act.rooms > 0 && (
        <div className="mb-5">
          <div className="mb-2 flex justify-between text-xs text-white/45">
            <span>Заполнено <b className="text-white/85">{act.filled} из {act.rooms}</b></span>
            <span className="font-bold text-[#FFB273] tnum">{act.percent}%</span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-white/8">
            <motion.i
              initial={{ width: 0 }}
              animate={{ width: `${act.percent}%` }}
              transition={{ type: 'spring', stiffness: 70, damping: 20, delay: 0.2 }}
              className="block h-full rounded-full bg-gradient-to-r from-[#FF9838] to-[#E33D6F] shadow-[0_0_12px_rgba(255,122,26,.6)]"
            />
          </div>
        </div>
      )}

      <div className="mb-5 flex flex-wrap gap-2 text-xs font-semibold">
        <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-white/60">
          <b className="text-white/90 tnum">{act.defects}</b> дефектов
        </span>
        <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1.5 text-white/60">
          <b className="text-white/90 tnum">{act.photos}</b> фото
        </span>
        {act.cloud_state === 'queue' && (
          <span className="inline-flex items-center gap-1.5 rounded-full border border-[#FFC24B]/30 bg-[#FFC24B]/10 px-3 py-1.5 text-[#FFC24B]">
            <span className="size-1.5 animate-pulse rounded-full bg-current motion-reduce:animate-none" />
            {act.cloud_n} в очереди
          </span>
        )}
        {act.cloud_state === 'error' && (
          <span className="rounded-full border border-[#FF7B7B]/30 bg-[#FF7B7B]/10 px-3 py-1.5 text-[#FF7B7B]">⚠ сбой выгрузки</span>
        )}
        {act.cloud_state === 'ok' && (
          <span className="rounded-full border border-[#5DD397]/30 bg-[#5DD397]/10 px-3 py-1.5 text-[#5DD397]">✓ выгружено</span>
        )}
      </div>

      <div className="flex gap-3">
        <motion.a
          href={`/inspections/${act.id}/edit`}
          whileTap={{ scale: 0.97 }}
          className="flex-1 rounded-xl bg-gradient-to-r from-[#FF8A2A] to-[#F0632F] py-3 text-center text-sm font-extrabold text-white shadow-[0_4px_20px_rgba(255,122,26,.3)]"
        >
          Продолжить
        </motion.a>
        <a
          href={`/inspections/${act.id}`}
          className="flex-1 rounded-xl border border-white/12 bg-white/5 py-3 text-center text-sm font-bold text-white/80 transition-colors hover:bg-white/10"
        >
          Просмотр
        </a>
      </div>
    </motion.article>
  )
}

function GlassSkeleton() {
  return <div className="h-64 animate-pulse rounded-3xl border border-white/8 bg-white/4 motion-reduce:animate-none" />
}
