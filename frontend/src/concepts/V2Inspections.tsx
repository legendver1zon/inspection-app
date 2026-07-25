import { useDeferredValue, useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api, type ActCard, type User } from '../lib/api'

/* Концепт V2 «Поле»: master-detail. Слева — компактный список актов
   по группам, справа — панель предпросмотра с кольцом прогресса.
   Тёплая палитра на токенах темы (работает светлая/тёмная). */

export default function V2Inspections({ user }: { user: User }) {
  const [q, setQ] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const deferredQ = useDeferredValue(q)

  const { data, isLoading } = useQuery({
    queryKey: ['inspections', deferredQ, 1],
    queryFn: () => api.inspections({ q: deferredQ, page: 1 }),
    placeholderData: keepPreviousData,
  })

  const all = data ? [...data.drafts, ...data.completed] : []
  const selected = all.find((a) => a.id === selectedId) ?? all[0] ?? null

  // При смене данных выбор остаётся валидным
  useEffect(() => {
    if (data && selectedId && !all.some((a) => a.id === selectedId)) setSelectedId(null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  const isAdmin = user.role === 'admin'

  return (
    <motion.main
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="mx-auto max-w-6xl px-4 pt-6 pb-24 sm:px-6"
    >
      <div className="mb-5 flex items-center gap-4">
        <h1 className="text-[26px] font-extrabold tracking-tight text-ink">Осмотры</h1>
        <div className="flex-1" />
        <motion.a
          href="/inspections/new"
          whileHover={{ y: -1 }}
          whileTap={{ scale: 0.97 }}
          className="inline-flex items-center gap-2 rounded-xl bg-accent px-5 py-3 text-sm font-extrabold text-on-accent shadow-[0_5px_16px_rgba(194,65,12,.28)] transition-colors hover:bg-accent-hover"
        >
          ＋ Новый осмотр
        </motion.a>
      </div>

      <div className="grid gap-5 lg:grid-cols-[400px_1fr]">
        {/* ===== Левая колонка: список ===== */}
        <div>
          <div className="relative mb-4">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"
                 className="pointer-events-none absolute top-1/2 left-3.5 -translate-y-1/2 text-faint" aria-hidden="true">
              <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
            </svg>
            <input
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Номер, адрес, фамилия"
              aria-label="Поиск по актам"
              className="w-full rounded-xl border border-line bg-surface py-3 pr-4 pl-10 text-sm text-ink shadow-[0_1px_2px_rgba(43,39,33,.05)] placeholder-faint focus:border-accent focus:shadow-[0_0_0_3px_rgba(194,65,12,.15)] focus:outline-none"
            />
          </div>

          {isLoading ? (
            <div className="grid gap-2">
              {[1, 2, 3, 4].map((i) => <div key={i} className="h-16 animate-pulse rounded-xl bg-inset motion-reduce:animate-none" />)}
            </div>
          ) : (
            <>
              <ListGroup
                label="В работе" tone="draft" count={data?.draft_count ?? 0}
                acts={data?.drafts ?? []} selected={selected} onSelect={setSelectedId}
              />
              <ListGroup
                label="Завершённые" tone="done" count={data?.completed_count ?? 0}
                acts={data?.completed ?? []} selected={selected} onSelect={setSelectedId}
              />
              {all.length === 0 && (
                <p className="py-8 text-center text-sm text-muted">
                  {deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Актов пока нет'}
                </p>
              )}
            </>
          )}
        </div>

        {/* ===== Правая панель: предпросмотр ===== */}
        <div className="hidden lg:block">
          <div className="sticky top-20">
            <AnimatePresence mode="wait">
              {selected ? (
                <DetailPanel key={selected.id} act={selected} isAdmin={isAdmin} />
              ) : (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  className="grid h-80 place-items-center rounded-3xl border border-line bg-surface text-muted"
                >
                  Выберите акт из списка
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </div>
      </div>

      {/* FAB на мобильном */}
      <a
        href="/inspections/new"
        className="fixed right-4 bottom-4 z-40 inline-flex min-h-13 items-center rounded-2xl bg-accent px-5 py-3.5 font-extrabold text-on-accent shadow-[0_10px_26px_rgba(194,65,12,.4)] sm:hidden"
      >
        ＋ Осмотр
      </a>
    </motion.main>
  )
}

/* ===== Группа в списке ===== */

function ListGroup({ label, tone, count, acts, selected, onSelect }: {
  label: string
  tone: 'draft' | 'done'
  count: number
  acts: ActCard[]
  selected: ActCard | null
  onSelect: (id: number) => void
}) {
  if (acts.length === 0) return null
  const ink = tone === 'draft' ? 'text-col-draft-ink' : 'text-col-done-ink'
  const dot = tone === 'draft' ? 'bg-[#C99A25]' : 'bg-[#4E9560]'
  return (
    <section className="mb-5">
      <div className="mb-2 flex items-center gap-2 px-1">
        <span className={`size-2 rounded-full ${dot}`} aria-hidden="true" />
        <h2 className={`text-[11.5px] font-extrabold tracking-[.11em] uppercase ${ink}`}>{label}</h2>
        <span className="text-[11.5px] font-bold text-faint tnum">{count}</span>
      </div>
      <div className="grid gap-1.5">
        {acts.map((act) => {
          const active = selected?.id === act.id
          return (
            <motion.button
              key={act.id}
              onClick={() => onSelect(act.id)}
              whileTap={{ scale: 0.99 }}
              className={`relative w-full cursor-pointer rounded-xl border p-3 text-left transition-colors ${
                active ? 'border-transparent' : 'border-line bg-surface hover:border-muted/60'
              }`}
            >
              {active && (
                <motion.span
                  layoutId="v2-selection"
                  className="absolute inset-0 rounded-xl bg-tonal"
                  transition={{ type: 'spring', stiffness: 450, damping: 38 }}
                />
              )}
              <span className="relative block">
                <span className="flex items-baseline gap-2.5">
                  <span className={`font-mono text-[14px] font-extrabold tnum ${active ? 'text-tonal-ink' : 'text-ink'}`}>
                    {act.act_number}
                  </span>
                  <span className="ml-auto text-[11.5px] text-muted">{act.date}</span>
                </span>
                <span className={`mt-0.5 line-clamp-1 block text-[13px] font-medium ${active ? 'text-tonal-ink' : 'text-ink'}`}>
                  {act.address || 'Адрес не указан'}
                </span>
                <span className="mt-1.5 flex items-center gap-2">
                  {act.status === 'draft' && act.rooms > 0 && (
                    <span className="h-1 w-24 overflow-hidden rounded-full bg-track">
                      <i className="block h-full rounded-full bg-accent-bright" style={{ width: `${act.percent}%` }} />
                    </span>
                  )}
                  <span className="text-[11.5px] text-muted tnum">{act.defects} деф · {act.photos} фото</span>
                  {act.cloud_state === 'queue' && <span className="size-1.5 animate-pulse rounded-full bg-warn motion-reduce:animate-none" title={`${act.cloud_n} фото в очереди`} />}
                  {act.cloud_state === 'error' && <span className="text-[11px] font-bold text-err">⚠</span>}
                </span>
              </span>
            </motion.button>
          )
        })}
      </div>
    </section>
  )
}

/* ===== Панель предпросмотра ===== */

function DetailPanel({ act, isAdmin }: { act: ActCard; isAdmin: boolean }) {
  const done = act.status === 'completed'
  return (
    <motion.article
      initial={{ opacity: 0, x: 14 }}
      animate={{ opacity: 1, x: 0 }}
      exit={{ opacity: 0, x: -8 }}
      transition={{ duration: 0.18 }}
      className="rounded-3xl border border-line bg-surface p-7 shadow-[0_1px_2px_rgba(43,39,33,.05),0_16px_40px_rgba(43,39,33,.07)]"
    >
      <div className="mb-1 flex items-center gap-3">
        <span className="font-mono text-[24px] font-extrabold tracking-tight text-ink tnum">№ {act.act_number}</span>
        {done ? (
          <span className="rounded-full bg-ok-bg px-3 py-1 text-xs font-extrabold text-ok uppercase">Завершён</span>
        ) : (
          <span className="rounded-full bg-warn-bg px-3 py-1 text-xs font-extrabold text-warn uppercase">В работе</span>
        )}
        <span className="ml-auto text-sm text-muted">{act.date}</span>
      </div>

      <div className="text-[17px] font-semibold text-ink">{act.address || 'Адрес не указан'}</div>
      <div className="mb-6 text-sm text-muted">
        {act.owner_name || 'Собственник не указан'}
        {act.total_area > 0 && <> · {act.total_area} м²</>}
        {isAdmin && act.inspector && <> · {act.inspector}</>}
      </div>

      <div className="mb-6 flex items-center gap-7">
        {/* Кольцо прогресса */}
        <ProgressRing percent={done ? 100 : act.percent} done={done} />
        <div className="grid flex-1 gap-2.5">
          <DetailRow label="Помещения" value={act.rooms > 0 ? `${act.filled} из ${act.rooms} заполнено` : '—'} />
          <DetailRow label="Дефекты" value={String(act.defects)} />
          <DetailRow label="Фотографии" value={String(act.photos)} />
          <DetailRow
            label="Облако"
            value={
              act.cloud_state === 'queue' ? `${act.cloud_n} фото в очереди` :
              act.cloud_state === 'error' ? 'сбой выгрузки' :
              act.cloud_state === 'ok' ? 'всё выгружено' : '—'
            }
            tone={act.cloud_state === 'queue' ? 'warn' : act.cloud_state === 'error' ? 'err' : act.cloud_state === 'ok' ? 'ok' : undefined}
          />
        </div>
      </div>

      <div className="flex gap-2.5">
        {!done && (
          <motion.a
            href={`/inspections/${act.id}/edit`}
            whileTap={{ scale: 0.97 }}
            className="flex-1 rounded-xl bg-accent py-3.5 text-center text-sm font-extrabold text-on-accent shadow-[0_5px_16px_rgba(194,65,12,.25)] transition-colors hover:bg-accent-hover"
          >
            Продолжить заполнение
          </motion.a>
        )}
        <a
          href={`/inspections/${act.id}`}
          className={`rounded-xl border border-line py-3.5 text-center text-sm font-bold text-ink transition-colors hover:border-muted ${done ? 'flex-1' : 'px-6'}`}
        >
          {done ? 'Открыть акт' : 'Просмотр'}
        </a>
        {done && (
          <a
            href={`/inspections/${act.id}`}
            className="rounded-xl bg-tonal px-6 py-3.5 text-center text-sm font-extrabold text-tonal-ink transition-colors hover:brightness-95"
          >
            PDF
          </a>
        )}
      </div>
    </motion.article>
  )
}

function ProgressRing({ percent, done }: { percent: number; done: boolean }) {
  const r = 52
  const c = 2 * Math.PI * r
  return (
    <div className="relative grid size-36 shrink-0 place-items-center">
      <svg width="144" height="144" viewBox="0 0 144 144" className="-rotate-90">
        <circle cx="72" cy="72" r={r} fill="none" strokeWidth="10" className="stroke-track" />
        <motion.circle
          cx="72" cy="72" r={r} fill="none" strokeWidth="10" strokeLinecap="round"
          className={done ? 'stroke-[#4E9560]' : 'stroke-accent-bright'}
          strokeDasharray={c}
          initial={{ strokeDashoffset: c }}
          animate={{ strokeDashoffset: c * (1 - percent / 100) }}
          transition={{ type: 'spring', stiffness: 60, damping: 20, delay: 0.1 }}
        />
      </svg>
      <div className="absolute grid place-items-center text-center">
        <span className="text-[26px] font-extrabold text-ink tnum">{percent}%</span>
        <span className="text-[10.5px] font-bold tracking-wider text-muted uppercase">{done ? 'готово' : 'заполнено'}</span>
      </div>
    </div>
  )
}

function DetailRow({ label, value, tone }: { label: string; value: string; tone?: 'ok' | 'warn' | 'err' }) {
  const color = tone === 'ok' ? 'text-ok' : tone === 'warn' ? 'text-warn' : tone === 'err' ? 'text-err' : 'text-ink'
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-line-soft pb-2 last:border-0">
      <span className="text-[12px] font-bold tracking-wide text-muted uppercase">{label}</span>
      <span className={`text-sm font-semibold ${color}`}>{value}</span>
    </div>
  )
}
