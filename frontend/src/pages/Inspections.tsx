import { useDeferredValue, useState } from 'react'
import { AnimatePresence, motion } from 'framer-motion'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api, type ActCard, type User } from '../lib/api'

export default function Inspections({ user }: { user: User }) {
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [mobileTab, setMobileTab] = useState<'draft' | 'done'>('draft')
  const deferredQ = useDeferredValue(q)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['inspections', deferredQ, page],
    queryFn: () => api.inspections({ q: deferredQ, page }),
    placeholderData: keepPreviousData,
  })

  const isAdmin = user.role === 'admin'

  return (
    <motion.main
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25 }}
      className="mx-auto max-w-6xl px-4 pt-6 pb-28 sm:px-6"
    >
      <div className="mb-5 flex items-center gap-4">
        <h1 className="text-[26px] font-extrabold tracking-tight text-ink">Осмотры</h1>
        <div className="flex-1" />
        <motion.a
          href="/inspections/new"
          whileHover={{ y: -1 }}
          whileTap={{ scale: 0.97 }}
          className="hidden items-center gap-2 rounded-xl bg-accent px-5 py-3 text-sm font-extrabold text-on-accent shadow-[0_5px_16px_rgba(194,65,12,.28)] transition-colors hover:bg-accent-hover sm:inline-flex"
        >
          ＋ Новый осмотр
        </motion.a>
      </div>

      {/* Поиск */}
      <div className="mb-5">
        <div className="relative">
          <SearchIcon />
          <input
            type="search"
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setPage(1)
            }}
            placeholder="Найти акт: номер, адрес, фамилия"
            aria-label="Поиск по актам"
            className="w-full rounded-xl border border-line bg-surface py-3.5 pr-4 pl-11 text-[15px] text-ink shadow-[0_1px_2px_rgba(43,39,33,.05)] placeholder-faint transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(194,65,12,.15)] focus:outline-none"
          />
        </div>
      </div>

      {/* Мобильные табы */}
      <div className="mb-4 flex gap-1 rounded-xl bg-inset p-1 sm:hidden" role="tablist">
        {(['draft', 'done'] as const).map((tab) => (
          <button
            key={tab}
            role="tab"
            aria-selected={mobileTab === tab}
            onClick={() => setMobileTab(tab)}
            className={`relative min-h-11 flex-1 cursor-pointer rounded-lg text-sm font-bold transition-colors ${
              mobileTab === tab ? 'text-ink' : 'text-muted'
            }`}
          >
            {mobileTab === tab && (
              <motion.span
                layoutId="tab-pill"
                className="absolute inset-0 rounded-lg bg-surface shadow-[0_1px_2px_rgba(43,39,33,.07)]"
                transition={{ type: 'spring', stiffness: 500, damping: 35 }}
              />
            )}
            <span className="relative">
              {tab === 'draft' ? 'В работе' : 'Завершённые'} · {tab === 'draft' ? (data?.draft_count ?? '…') : (data?.completed_count ?? '…')}
            </span>
          </button>
        ))}
      </div>

      {isError && (
        <div className="rounded-2xl bg-err-bg px-5 py-4 font-semibold text-err">
          Не удалось загрузить список. Обновите страницу.
        </div>
      )}

      {/* В работе */}
      <section
        aria-labelledby="h-draft"
        className={`mb-5 rounded-[18px] border border-col-draft-line bg-col-draft p-4 ${mobileTab !== 'draft' ? 'hidden sm:block' : ''}`}
      >
        <GroupHead id="h-draft" title="В работе" count={data?.draft_count} tone="draft" />
        {isLoading ? (
          <div className="grid gap-3 sm:grid-cols-2">
            <Skeleton h={220} />
            <Skeleton h={220} />
          </div>
        ) : data && data.drafts.length > 0 ? (
          <motion.div
            variants={{ show: { transition: { staggerChildren: 0.06 } } }}
            initial="hidden"
            animate="show"
            className="grid gap-3 sm:grid-cols-2"
          >
            <AnimatePresence mode="popLayout">
              {data.drafts.map((act) => (
                <DraftCard key={act.id} act={act} isAdmin={isAdmin} />
              ))}
            </AnimatePresence>
          </motion.div>
        ) : (
          <Empty
            title={deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Пока нет актов в работе'}
            hint={deferredQ ? 'Проверьте номер, адрес или фамилию' : 'Создайте первый осмотр — это займёт минуту'}
            action={
              deferredQ ? (
                <button onClick={() => setQ('')} className="cursor-pointer rounded-xl border border-line bg-surface px-4 py-2.5 text-sm font-bold text-ink hover:border-muted">
                  Сбросить поиск
                </button>
              ) : (
                <a href="/inspections/new" className="rounded-xl bg-accent px-4 py-2.5 text-sm font-extrabold text-on-accent hover:bg-accent-hover">
                  ＋ Новый осмотр
                </a>
              )
            }
          />
        )}
      </section>

      {/* Завершённые */}
      <section
        aria-labelledby="h-done"
        className={`rounded-[18px] border border-col-done-line bg-col-done p-4 ${mobileTab !== 'done' ? 'hidden sm:block' : ''}`}
      >
        <GroupHead id="h-done" title="Завершённые" count={data?.completed_count} tone="done" />
        {isLoading ? (
          <Skeleton h={200} />
        ) : data && data.completed.length > 0 ? (
          <>
            {/* Таблица (desktop) */}
            <div className="hidden overflow-x-auto rounded-xl border border-line bg-surface shadow-[0_1px_2px_rgba(43,39,33,.05)] sm:block">
              <table className="w-full min-w-[880px] border-collapse text-sm">
                <thead>
                  <tr className="border-b border-line text-left text-[11px] font-bold tracking-wider text-muted uppercase">
                    <th className="px-3.5 py-3">№</th>
                    <th className="px-3.5 py-3">Дата</th>
                    <th className="px-3.5 py-3">Адрес</th>
                    <th className="px-3.5 py-3">Собственник</th>
                    {isAdmin && <th className="px-3.5 py-3">Инспектор</th>}
                    <th className="px-3.5 py-3 text-right">м²</th>
                    <th className="px-3.5 py-3 text-right">Дефекты</th>
                    <th className="px-3.5 py-3 text-right">Фото</th>
                    <th className="px-3.5 py-3">Статус</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {data.completed.map((act) => (
                    <motion.tr
                      key={act.id}
                      initial={{ opacity: 0 }}
                      animate={{ opacity: 1 }}
                      className="border-b border-line-soft last:border-0 hover:bg-inset/60"
                    >
                      <td className="px-3.5 py-3 font-mono font-bold tnum">
                        <a href={`/inspections/${act.id}`} className="text-ink hover:text-accent">{act.act_number}</a>
                      </td>
                      <td className="px-3.5 py-3 whitespace-nowrap text-muted">{act.date}</td>
                      <td className="px-3.5 py-3">{act.address}</td>
                      <td className="px-3.5 py-3 text-muted">{act.owner_name}</td>
                      {isAdmin && <td className="px-3.5 py-3 text-muted">{act.inspector}</td>}
                      <td className="px-3.5 py-3 text-right tnum">{act.total_area > 0 ? act.total_area : '—'}</td>
                      <td className="px-3.5 py-3 text-right tnum">{act.defects}</td>
                      <td className="px-3.5 py-3 text-right tnum">{act.photos}</td>
                      <td className="px-3.5 py-3"><DoneStamp /></td>
                      <td className="px-3.5 py-3">
                        <a href={`/inspections/${act.id}`} className="rounded-lg px-2.5 py-2 font-semibold whitespace-nowrap text-muted hover:bg-inset hover:text-accent">
                          Открыть
                        </a>
                      </td>
                    </motion.tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Карточки (mobile) */}
            <div className="grid gap-2.5 sm:hidden">
              {data.completed.map((act) => (
                <article key={act.id} className="grid gap-2 rounded-xl border border-line bg-surface p-3.5">
                  <div className="flex items-baseline gap-2.5">
                    <a href={`/inspections/${act.id}`} className="font-mono text-[15px] font-bold text-ink tnum">№ {act.act_number}</a>
                    <span className="ml-auto text-xs text-muted">{act.date}</span>
                  </div>
                  <div className="text-sm font-semibold">{act.address}</div>
                  <div className="flex flex-wrap gap-x-3 text-[13px] text-muted">
                    <span>{act.owner_name}</span>
                    <span><b className="text-ink tnum">{act.defects}</b> дефектов</span>
                    <span><b className="text-ink tnum">{act.photos}</b> фото</span>
                  </div>
                  <div className="flex items-center gap-2 border-t border-line-soft pt-2.5">
                    <DoneStamp />
                    <a href={`/inspections/${act.id}`} className="ml-auto rounded-lg border border-line px-4 py-2 text-[13px] font-bold text-ink hover:border-muted">
                      Открыть
                    </a>
                  </div>
                </article>
              ))}
            </div>

            <div className="flex items-center gap-3 px-1 pt-3 text-[13px] text-muted">
              <span>Показано {data.completed.length} из {data.completed_count}</span>
              {data.total_pages > 1 && (
                <nav className="ml-auto flex items-center gap-1" aria-label="Страницы">
                  <PagerBtn disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>‹</PagerBtn>
                  <span className="grid min-h-9 min-w-9 place-items-center rounded-lg bg-tonal font-bold text-tonal-ink tnum">{page}</span>
                  <PagerBtn disabled={page >= data.total_pages} onClick={() => setPage((p) => p + 1)}>›</PagerBtn>
                </nav>
              )}
            </div>
          </>
        ) : (
          <Empty
            title={deferredQ ? `Ничего не найдено по запросу «${deferredQ}»` : 'Завершённых актов пока нет'}
            hint={deferredQ ? '' : 'Заполненный и подписанный акт появится здесь'}
          />
        )}
      </section>

      {/* FAB на мобильном */}
      <motion.a
        href="/inspections/new"
        initial={{ scale: 0, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        transition={{ type: 'spring', stiffness: 320, damping: 20, delay: 0.2 }}
        whileTap={{ scale: 0.94 }}
        className="fixed right-4 bottom-4 z-40 inline-flex min-h-13 items-center rounded-2xl bg-accent px-5 py-3.5 font-extrabold text-on-accent shadow-[0_10px_26px_rgba(194,65,12,.4)] sm:hidden"
      >
        ＋ Осмотр
      </motion.a>
    </motion.main>
  )
}

/* ===== Карточка черновика ===== */

function DraftCard({ act, isAdmin }: { act: ActCard; isAdmin: boolean }) {
  return (
    <motion.article
      layout
      variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
      exit={{ opacity: 0, scale: 0.96 }}
      whileHover={{ y: -2 }}
      transition={{ type: 'spring', stiffness: 350, damping: 26 }}
      className="grid content-start gap-2.5 rounded-2xl border border-line bg-surface p-4 pb-3 shadow-[0_1px_2px_rgba(43,39,33,.05),0_10px_26px_rgba(43,39,33,.06)]"
    >
      <div className="flex items-baseline gap-2.5">
        <a href={`/inspections/${act.id}`} className="font-mono text-[17px] font-extrabold text-ink transition-colors tnum hover:text-accent">
          № {act.act_number}
        </a>
        <span className="ml-auto text-[13px] whitespace-nowrap text-muted">{act.date}</span>
      </div>

      <div className="text-[15px] leading-snug font-semibold">{act.address}</div>
      <div className="-mt-1 text-[13px] text-muted">
        {act.owner_name || 'Собственник не указан'}
        {act.rooms > 0 && <> · {act.rooms} пом.</>}
        {act.total_area > 0 && <> · {act.total_area} м²</>}
        {isAdmin && act.inspector && <> · {act.inspector}</>}
      </div>

      {act.rooms > 0 && (
        <div className="grid gap-1.5">
          <div className="flex justify-between text-xs text-muted">
            <span>
              Заполнено <b className="font-semibold text-ink">{act.filled} из {act.rooms}</b> помещений
            </span>
            <span className="tnum">{act.percent}%</span>
          </div>
          <div
            className="h-1.5 overflow-hidden rounded-full bg-track"
            role="progressbar"
            aria-valuenow={act.filled}
            aria-valuemin={0}
            aria-valuemax={act.rooms}
            aria-label={`Заполнено ${act.filled} из ${act.rooms} помещений`}
          >
            <motion.i
              initial={{ width: 0 }}
              animate={{ width: `${act.percent}%` }}
              transition={{ type: 'spring', stiffness: 80, damping: 20, delay: 0.15 }}
              className="block h-full rounded-full bg-gradient-to-r from-[#F79240] to-accent-bright"
            />
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-1.5">
        <Fact icon={<ChecklistIcon />} label={`${act.defects} дефектов`}>
          <b className="font-bold text-ink tnum">{act.defects}</b> дефектов
        </Fact>
        <Fact icon={<CameraIcon />} label={`${act.photos} фото`}>
          <b className="font-bold text-ink tnum">{act.photos}</b> фото
        </Fact>
        {act.cloud_state === 'queue' && (
          <span role="status" aria-label={`${act.cloud_n} фото в очереди на выгрузку`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-warn-bg px-2.5 py-1.5 text-xs font-semibold text-warn">
            <span className="size-1.5 animate-pulse rounded-full bg-current motion-reduce:animate-none" />
            {act.cloud_n} в очереди
          </span>
        )}
        {act.cloud_state === 'error' && (
          <span aria-label={`Сбой выгрузки ${act.cloud_n} фото`}
                className="inline-flex items-center gap-1.5 rounded-lg bg-err-bg px-2.5 py-1.5 text-xs font-semibold text-err">
            <WarnIcon /> сбой выгрузки
          </span>
        )}
        {act.cloud_state === 'ok' && (
          <span aria-label="Все фото выгружены в облако"
                className="inline-flex items-center gap-1.5 rounded-lg bg-ok-bg px-2.5 py-1.5 text-xs font-semibold text-ok">
            <CloudOkIcon /> выгружено
          </span>
        )}
      </div>

      <div className="flex gap-2 border-t border-line-soft pt-3">
        <motion.a
          href={`/inspections/${act.id}/edit`}
          whileTap={{ scale: 0.97 }}
          className="flex-1 rounded-[10px] bg-tonal py-3 text-center text-[13.5px] font-bold text-tonal-ink transition-colors hover:brightness-95"
        >
          Продолжить
        </motion.a>
        <a
          href={`/inspections/${act.id}`}
          className="flex-1 rounded-[10px] border border-line py-3 text-center text-[13.5px] font-bold text-ink transition-colors hover:border-muted"
        >
          Просмотр
        </a>
      </div>
    </motion.article>
  )
}

/* ===== Мелкие компоненты ===== */

function GroupHead({ id, title, count, tone }: { id: string; title: string; count?: number; tone: 'draft' | 'done' }) {
  const ink = tone === 'draft' ? 'text-col-draft-ink' : 'text-col-done-ink'
  const dot = tone === 'draft' ? 'bg-[#C99A25]' : 'bg-[#4E9560]'
  return (
    <div className="flex items-center gap-2.5 px-1.5 pb-3">
      <span className={`size-2.5 rounded-full ${dot}`} aria-hidden="true" />
      <h2 id={id} className={`text-[12.5px] font-extrabold tracking-[.11em] uppercase ${ink}`}>{title}</h2>
      <span className={`ml-auto rounded-[7px] bg-surface/70 px-2.5 py-1 text-[12.5px] font-extrabold tnum ${ink}`}>
        {count ?? '…'}
      </span>
    </div>
  )
}

function Fact({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  return (
    <span aria-label={label} className="inline-flex items-center gap-1.5 rounded-lg border border-line-soft bg-inset px-2.5 py-1.5 text-xs font-semibold text-muted">
      <span className="text-faint">{icon}</span>
      {children}
    </span>
  )
}

function DoneStamp() {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-lg bg-ok-bg px-2.5 py-1.5 text-xs font-bold whitespace-nowrap text-ok">
      <CheckIcon /> Завершён
    </span>
  )
}

function Empty({ title, hint, action }: { title: string; hint?: string; action?: React.ReactNode }) {
  return (
    <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} className="grid justify-items-center gap-3 px-4 py-8 text-center">
      <div className="text-[15px] font-semibold text-ink">{title}</div>
      {hint && <div className="text-sm text-muted">{hint}</div>}
      {action}
    </motion.div>
  )
}

function Skeleton({ h }: { h: number }) {
  return <div style={{ height: h }} className="animate-pulse rounded-2xl border border-line-soft bg-inset motion-reduce:animate-none" />
}

function PagerBtn({ disabled, onClick, children }: { disabled: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      disabled={disabled}
      onClick={onClick}
      className="grid min-h-9 min-w-9 cursor-pointer place-items-center rounded-lg border border-line bg-surface font-semibold text-ink transition-colors hover:border-muted disabled:cursor-default disabled:opacity-40"
    >
      {children}
    </button>
  )
}

/* ===== Иконки ===== */

function SearchIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4"
         className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-faint" aria-hidden="true">
      <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" />
    </svg>
  )
}
function ChecklistIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="M9 6h11M9 12h11M9 18h11" /><path d="m3.5 5.5 1 1L6.5 4.5m-3 7 1 1 2-2m-3 7 1 1 2-2" />
    </svg>
  )
}
function CameraIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <rect x="3" y="7" width="18" height="13" rx="2" /><path d="m8 7 2-3h4l2 3" /><circle cx="12" cy="13" r="3.5" />
    </svg>
  )
}
function WarnIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="m12 3 10 18H2Z" /><path d="M12 10v5" /><circle cx="12" cy="18" r=".5" />
    </svg>
  )
}
function CloudOkIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="M6 18a5 5 0 0 1 .6-9.97A6.5 6.5 0 0 1 19 10a4 4 0 0 1-1 7.87" /><path d="m8.5 14.5 2.5 2.5 4.5-5" />
    </svg>
  )
}
function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" aria-hidden="true">
      <circle cx="12" cy="12" r="9" /><path d="m8.5 12.5 2.5 2.5 4.5-5.5" />
    </svg>
  )
}
