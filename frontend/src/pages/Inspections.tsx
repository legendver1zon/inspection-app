import { useDeferredValue, useMemo, useState } from 'react'
import { motion } from 'framer-motion'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api, type ActCard, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'

/* Концепт V1 «Лента»: хронология выездов. Акты сгруппированы по дням
   на вертикальной оси, сверху — сводка большими числами. Светлая бумага,
   чернильный, глубокий тил. Палитра — lib/palette. */


type Filter = 'all' | 'draft' | 'completed'

export default function V1Inspections({ user }: { user: User }) {
  const [q, setQ] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const deferredQ = useDeferredValue(q)

  const { data, isLoading } = useQuery({
    queryKey: ['inspections', deferredQ, 1],
    queryFn: () => api.inspections({ q: deferredQ, page: 1 }),
    placeholderData: keepPreviousData,
  })

  // Лента: все акты вместе, по дням, новые сверху
  const days = useMemo(() => {
    if (!data) return []
    let acts = [...data.drafts, ...data.completed]
    if (filter !== 'all') acts = acts.filter((a) => a.status === filter)
    const byDay = new Map<string, ActCard[]>()
    for (const act of acts) {
      const key = act.date || 'Без даты'
      if (!byDay.has(key)) byDay.set(key, [])
      byDay.get(key)!.push(act)
    }
    return [...byDay.entries()]
  }, [data, filter])

  const queueTotal = useMemo(
    () => (data ? [...data.drafts, ...data.completed].reduce((s, a) => s + (a.cloud_state === 'queue' ? a.cloud_n : 0), 0) : 0),
    [data],
  )

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />

      <main className="mx-auto max-w-5xl px-5 pt-8 pb-28">
        {/* Сводка большими числами */}
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          className="mb-8 grid grid-cols-3 gap-4"
        >
          <Stat n={data?.draft_count} label="в работе" accent />
          <Stat n={data?.completed_count} label="завершено" />
          <Stat n={data ? queueTotal : undefined} label="фото в очереди" warn={queueTotal > 0} />
        </motion.div>

        {/* Поиск + фильтр */}
        <div className="mb-10 flex flex-wrap items-center gap-3">
          <div className="relative min-w-60 flex-1">
            <span className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2" style={{ color: C.faint }}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
            </span>
            <input
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Найти акт: номер, адрес, фамилия"
              className="w-full rounded-full border py-3 pr-5 pl-11 text-[14.5px] outline-none"
              style={{ background: C.surface, borderColor: C.line, color: C.ink }}
            />
          </div>
          <div className="flex rounded-full border p-1" style={{ background: C.surface, borderColor: C.line }}>
            {([['all', 'Все'], ['draft', 'В работе'], ['completed', 'Завершённые']] as [Filter, string][]).map(([key, label]) => (
              <button
                key={key}
                onClick={() => setFilter(key)}
                className="relative cursor-pointer rounded-full px-4 py-2 text-[13px] font-bold whitespace-nowrap"
                style={{ color: filter === key ? '#fff' : C.muted }}
              >
                {filter === key && (
                  <motion.span
                    layoutId="lenta-filter"
                    className="absolute inset-0 rounded-full"
                    style={{ background: C.accent }}
                    transition={{ type: 'spring', stiffness: 500, damping: 35 }}
                  />
                )}
                <span className="relative">{label}</span>
              </button>
            ))}
          </div>
          <a
            href="/inspections/new"
            className="rounded-full px-5 py-3 text-[14px] font-extrabold text-white transition-opacity hover:opacity-90"
            style={{ background: C.ink }}
          >
            ＋ Осмотр
          </a>
        </div>

        {/* Лента по дням */}
        {isLoading ? (
          <div className="grid gap-4">
            <div className="h-28 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />
            <div className="h-28 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />
          </div>
        ) : days.length > 0 ? (
          <div className="relative pl-8 sm:pl-36">
            {/* Ось */}
            <span aria-hidden="true" className="absolute top-2 bottom-2 left-2.5 w-px sm:left-[118px]" style={{ background: C.rail }} />
            <motion.div
              variants={{ show: { transition: { staggerChildren: 0.06 } } }}
              initial="hidden"
              animate="show"
              className="grid gap-8"
            >
              {days.map(([day, acts]) => (
                <motion.section key={day} variants={{ hidden: { opacity: 0, y: 16 }, show: { opacity: 1, y: 0 } }}>
                  {/* Узел дня */}
                  <div className="relative mb-3 flex items-center gap-3">
                    <span
                      aria-hidden="true"
                      className="absolute -left-[26px] size-3 rounded-full border-2 sm:-left-[23.5px]"
                      style={{ background: C.surface, borderColor: C.accent }}
                    />
                    <h2 className="text-[15px] font-extrabold tracking-tight sm:absolute sm:-left-36 sm:w-28 sm:text-right sm:text-[16px]">
                      {day}
                    </h2>
                  </div>
                  <div className="grid gap-3">
                    {acts.map((act) => <LentaCard key={act.id} act={act} />)}
                  </div>
                </motion.section>
              ))}
            </motion.div>
          </div>
        ) : (
          <p className="py-10 text-center" style={{ color: C.muted }}>
            {deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Актов пока нет — создайте первый осмотр'}
          </p>
        )}
      </main>
    </div>
  )
}

function Stat({ n, label, accent, warn }: { n?: number; label: string; accent?: boolean; warn?: boolean }) {
  const color = warn ? C.warn : accent ? C.accent : C.ink
  return (
    <div className="rounded-2xl border px-3.5 py-3 sm:px-5 sm:py-4" style={{ background: C.surface, borderColor: C.line }}>
      <div className="text-[28px] leading-none font-black tracking-tight tnum sm:text-[40px]" style={{ color }}>
        {n ?? '…'}
      </div>
      <div className="mt-1.5 text-[10.5px] leading-tight font-bold tracking-wide uppercase sm:text-[12px] sm:tracking-wider" style={{ color: C.faint }}>{label}</div>
    </div>
  )
}

function LentaCard({ act }: { act: ActCard }) {
  const done = act.status === 'completed'
  return (
    <motion.article
      whileHover={{ x: 3 }}
      transition={{ type: 'spring', stiffness: 400, damping: 28 }}
      className="rounded-2xl border p-4 sm:p-5"
      style={{ background: C.surface, borderColor: C.line, boxShadow: '0 1px 2px rgba(51,48,42,.05)' }}
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <a href={`/inspections/${act.id}`} className="font-mono text-[17px] font-extrabold hover:underline tnum" style={{ color: C.ink }}>
          {act.act_number}
        </a>
        {done ? (
          <span className="rounded-full px-2.5 py-1 text-[11.5px] font-extrabold uppercase" style={{ background: C.okBg, color: C.ok }}>
            Завершён
          </span>
        ) : (
          <span className="rounded-full px-2.5 py-1 text-[11.5px] font-extrabold uppercase" style={{ background: C.accentSoft, color: C.accentDark }}>
            В работе
          </span>
        )}
        {act.cloud_state === 'queue' && (
          <span className="inline-flex items-center gap-1.5 text-[12.5px] font-bold" style={{ color: C.warn }}>
            <span className="size-1.5 animate-pulse rounded-full bg-current motion-reduce:animate-none" />
            {act.cloud_n} фото в очереди
          </span>
        )}
        {act.cloud_state === 'error' && (
          <span className="text-[12.5px] font-bold" style={{ color: C.err }}>⚠ сбой выгрузки</span>
        )}
        <span className="ml-auto hidden text-[13px] sm:block tnum" style={{ color: C.faint }}>
          {act.defects} деф · {act.photos} фото{act.total_area > 0 && <> · {act.total_area} м²</>}
        </span>
      </div>

      <div className="mt-2 text-[15.5px] font-semibold">{act.address || 'Адрес не указан'}</div>
      <div className="text-[13.5px]" style={{ color: C.muted }}>{act.owner_name || 'Собственник не указан'}</div>

      <div className="mt-3 flex flex-wrap items-center gap-3">
        {!done && act.rooms > 0 && (
          <div className="w-full sm:max-w-72 sm:flex-1">
            <div className="mb-1 flex justify-between text-[11.5px]" style={{ color: C.muted }}>
              <span className="tnum">{act.filled}/{act.rooms} помещений</span>
              <span className="font-bold tnum" style={{ color: C.accentDark }}>{act.percent}%</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full" style={{ background: C.track }}>
              <motion.i
                initial={{ width: 0 }}
                animate={{ width: `${act.percent}%` }}
                transition={{ type: 'spring', stiffness: 90, damping: 20, delay: 0.1 }}
                className="block h-full rounded-full"
                style={{ background: C.accent }}
              />
            </div>
          </div>
        )}
        <div className="ml-auto flex gap-2">
          {!done && (
            <a
              href={`/inspections/${act.id}/edit`}
              className="rounded-full px-4 py-2 text-[13px] font-extrabold text-white transition-opacity hover:opacity-90"
              style={{ background: C.accent }}
            >
              Продолжить
            </a>
          )}
          <a
            href={`/inspections/${act.id}`}
            className="rounded-full border px-4 py-2 text-[13px] font-bold transition-colors"
            style={{ borderColor: C.line, color: C.ink }}
          >
            {done ? 'Открыть' : 'Просмотр'}
          </a>
        </div>
      </div>
    </motion.article>
  )
}
