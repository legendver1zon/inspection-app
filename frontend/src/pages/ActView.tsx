import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Defect, type PhotoRef, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'
import PhotoThumb from '../components/PhotoThumb'

/* Просмотр акта в языке «Ленты»: кремовая бумага, белые карточки,
   помещения — секциями вниз по странице, фото — плитками. */

export default function ActView({ user }: { user: User }) {
  const { id } = useParams()
  const actId = Number(id)
  const queryClient = useQueryClient()
  const [pdfBusy, setPdfBusy] = useState(false)
  const [statusBusy, setStatusBusy] = useState(false)

  const { data, isLoading, isError } = useQuery({
    queryKey: ['inspection', actId],
    queryFn: () => api.inspection(actId),
    enabled: Number.isFinite(actId),
  })

  const act = data?.inspection

  async function generatePdf() {
    setPdfBusy(true)
    try {
      await api.generatePdf(actId)
      await queryClient.invalidateQueries({ queryKey: ['inspection', actId] })
    } finally {
      setPdfBusy(false)
    }
  }

  async function toggleStatus() {
    if (!act) return
    setStatusBusy(true)
    try {
      await api.setStatus(actId, act.status === 'draft' ? 'completed' : 'draft')
      await queryClient.invalidateQueries()
    } finally {
      setStatusBusy(false)
    }
  }

  const totalDefects = act?.rooms.reduce((s, r) => s + r.defects.length, 0) ?? 0
  const totalPhotos = act?.rooms.reduce((s, r) => s + r.defects.reduce((ss, d) => ss + d.photos.length, 0), 0) ?? 0

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />

      <main className="mx-auto max-w-5xl px-5 pt-6 pb-24">
        <a href="/inspections" className="mb-5 inline-flex items-center gap-2 text-sm font-bold hover:underline" style={{ color: C.muted }}>
          ← К ленте осмотров
        </a>

        {isLoading && (
          <div className="grid gap-4">
            <div className="h-40 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />
            <div className="h-64 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />
          </div>
        )}
        {isError && (
          <div className="rounded-2xl px-5 py-4 font-semibold" style={{ background: C.errBg, color: C.err }}>
            Не удалось загрузить акт. Возможно, он удалён или у вас нет доступа.
          </div>
        )}

        {act && (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
            {/* ===== Шапка акта ===== */}
            <div className="mb-5 rounded-3xl border p-6 sm:p-7" style={{ background: C.surface, borderColor: C.line }}>
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
                <h1 className="font-mono text-[26px] font-extrabold tracking-tight tnum sm:text-[30px]">
                  № {act.act_number}
                </h1>
                {act.status === 'completed' ? (
                  <span className="rounded-full px-3 py-1.5 text-[11.5px] font-extrabold uppercase" style={{ background: C.okBg, color: C.ok }}>
                    Завершён
                  </span>
                ) : (
                  <span className="rounded-full px-3 py-1.5 text-[11.5px] font-extrabold uppercase" style={{ background: C.accentSoft, color: C.accentDark }}>
                    В работе
                  </span>
                )}
                <span className="ml-auto text-sm" style={{ color: C.muted }}>
                  {act.date}{act.time && ` · ${act.time}`}
                </span>
              </div>

              <div className="mt-3 text-[19px] font-bold">{act.address || 'Адрес не указан'}</div>
              <div className="mt-1 text-[14px]" style={{ color: C.muted }}>
                {act.owner_name && <>Собственник — {act.owner_name}</>}
                {act.developer_rep_name && <> · Застройщик — {act.developer_rep_name}</>}
                {act.inspector && <> · Инспектор — {act.inspector}</>}
              </div>

              {/* Параметры */}
              <div className="mt-5 flex flex-wrap gap-2">
                {[
                  act.rooms_count > 0 && `${act.rooms_count} комн.`,
                  act.floor > 0 && `${act.floor} этаж`,
                  act.total_area > 0 && `${act.total_area} м²`,
                  act.temp_outside !== 0 && `t° нар. ${act.temp_outside}°C`,
                  act.temp_inside !== 0 && `t° внутр. ${act.temp_inside}°C`,
                  act.humidity > 0 && `влажность ${act.humidity}%`,
                  act.electricity && `электричество: ${act.electricity}`,
                  act.ventilation && `вентиляция: ${act.ventilation}`,
                ]
                  .filter(Boolean)
                  .map((chip) => (
                    <span key={String(chip)} className="rounded-full border px-3 py-1.5 text-[12.5px] font-semibold" style={{ borderColor: C.line, color: C.muted }}>
                      {chip}
                    </span>
                  ))}
              </div>

              {/* Действия */}
              <div className="mt-6 flex flex-wrap gap-2.5">
                <a
                  href={`/inspections/${act.id}/edit`}
                  className="rounded-full px-5 py-2.5 text-[13.5px] font-extrabold text-white transition-opacity hover:opacity-90"
                  style={{ background: C.accent }}
                >
                  Редактировать
                </a>
                <button
                  onClick={generatePdf}
                  disabled={pdfBusy}
                  className="cursor-pointer rounded-full border px-5 py-2.5 text-[13.5px] font-bold transition-colors disabled:opacity-50"
                  style={{ borderColor: C.line, color: C.ink }}
                >
                  {pdfBusy ? 'Генерируем…' : 'Сформировать PDF'}
                </button>
                <button
                  onClick={toggleStatus}
                  disabled={statusBusy}
                  className="cursor-pointer rounded-full px-5 py-2.5 text-[13.5px] font-extrabold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                  style={{ background: act.status === 'draft' ? '#3D8B6E' : C.muted }}
                >
                  {statusBusy ? '…' : act.status === 'draft' ? '✓ Завершить акт' : 'Вернуть в работу'}
                </button>
                {act.photo_folder_url && (
                  <a
                    href={act.photo_folder_url}
                    target="_blank"
                    rel="noreferrer"
                    className="rounded-full border px-5 py-2.5 text-[13.5px] font-bold hover:underline"
                    style={{ borderColor: C.line, color: C.accentDark }}
                  >
                    Фото в облаке ↗
                  </a>
                )}
              </div>
            </div>

            {/* ===== Сводка ===== */}
            <div className="mb-8 grid grid-cols-3 gap-3">
              <MiniStat n={act.rooms.length} label="помещений" />
              <MiniStat n={totalDefects} label="дефектов" />
              <MiniStat n={totalPhotos} label="фото" />
            </div>

            {/* ===== Помещения ===== */}
            {act.rooms.length === 0 ? (
              <div className="rounded-2xl border px-5 py-10 text-center" style={{ background: C.surface, borderColor: C.line, color: C.muted }}>
                Помещения ещё не добавлены — начните с «Редактировать»
              </div>
            ) : (
              <motion.div
                variants={{ show: { transition: { staggerChildren: 0.05 } } }}
                initial="hidden"
                animate="show"
                className="grid gap-4"
              >
                {act.rooms.map((room) => (
                  <motion.section
                    key={room.id}
                    variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
                    className="rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}
                    aria-label={`Помещение ${room.name}`}
                  >
                    <div className="mb-3 flex items-baseline gap-3">
                      <span className="grid size-7 place-items-center rounded-lg text-[13px] font-extrabold text-white tnum" style={{ background: C.accent }}>
                        {room.number}
                      </span>
                      <h2 className="text-[17px] font-extrabold">{room.name || `Помещение ${room.number}`}</h2>
                      <span className="ml-auto text-[12.5px]" style={{ color: C.faint }}>
                        {room.defects.length > 0 ? `${room.defects.length} дефектов` : 'без дефектов'}
                      </span>
                    </div>

                    {room.defects.length > 0 && (
                      <div className="grid gap-2.5">
                        {room.defects.map((d) => <DefectRow key={d.id} defect={d} />)}
                      </div>
                    )}
                  </motion.section>
                ))}
              </motion.div>
            )}

            {/* ===== Архив ===== */}
            {act.archived.length > 0 && (
              <details className="mt-6 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
                <summary className="cursor-pointer text-[14px] font-bold" style={{ color: C.muted }}>
                  Архив: {act.archived.length} удалённых дефектов с фото (не попадают в PDF)
                </summary>
                <div className="mt-4 grid gap-2.5">
                  {act.archived.map((d, i) => (
                    <div key={i} className="rounded-xl border p-3.5" style={{ borderColor: C.line }}>
                      <div className="text-[13.5px] font-semibold" style={{ color: C.muted }}>
                        {d.room_name && `${d.room_name} · `}{d.name}{d.value && ` — ${d.value}`}
                      </div>
                      <PhotoStrip photos={d.photos} />
                    </div>
                  ))}
                </div>
              </details>
            )}

            {/* ===== Документы ===== */}
            <section className="mt-6 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }} aria-label="Документы">
              <h2 className="mb-3 text-[15px] font-extrabold">Документы</h2>
              {act.documents.length === 0 ? (
                <p className="text-sm" style={{ color: C.muted }}>
                  Пока нет — нажмите «Сформировать PDF», чтобы собрать акт с фотографиями и QR-кодом.
                </p>
              ) : (
                <div className="grid gap-2">
                  {act.documents.map((doc) => (
                    <div key={doc.id} className="flex items-center gap-3 rounded-xl border px-4 py-3" style={{ borderColor: C.line }}>
                      <span className="grid size-9 place-items-center rounded-lg text-[11px] font-extrabold uppercase" style={{ background: C.accentSoft, color: C.accentDark }}>
                        {doc.format}
                      </span>
                      <span className="text-sm font-semibold">Акт № {act.act_number}</span>
                      <span className="text-[12.5px]" style={{ color: C.faint }}>{doc.created}</span>
                      <a
                        href={`/documents/${doc.id}/download`}
                        className="ml-auto rounded-full px-4 py-2 text-[13px] font-extrabold text-white transition-opacity hover:opacity-90"
                        style={{ background: C.accent }}
                      >
                        Скачать
                      </a>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </motion.div>
        )}
      </main>
    </div>
  )
}

function MiniStat({ n, label }: { n: number; label: string }) {
  return (
    <div className="rounded-2xl border px-4 py-3" style={{ background: C.surface, borderColor: C.line }}>
      <div className="text-[24px] leading-none font-black tnum sm:text-[28px]">{n}</div>
      <div className="mt-1 text-[10.5px] font-bold tracking-wide uppercase" style={{ color: C.faint }}>{label}</div>
    </div>
  )
}

function DefectRow({ defect }: { defect: Defect }) {
  return (
    <div className="rounded-xl border p-3.5" style={{ borderColor: C.line, background: C.bg }}>
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <span className="rounded-md px-2 py-0.5 text-[10.5px] font-extrabold tracking-wide uppercase" style={{ background: C.accentSoft, color: C.accentDark }}>
          {defect.section_name || defect.section}
          {defect.wall_number > 0 && ` · ст. ${defect.wall_number}`}
        </span>
        <span className="text-[14px] font-bold">{defect.name}</span>
        {defect.value && <span className="text-[13.5px]" style={{ color: C.muted }}>{defect.value}</span>}
      </div>
      {defect.notes && <div className="mt-1 text-[13px]" style={{ color: C.muted }}>{defect.notes}</div>}
      <PhotoStrip photos={defect.photos} />
    </div>
  )
}

function PhotoStrip({ photos }: { photos: PhotoRef[] }) {
  if (photos.length === 0) return null
  return (
    <div className="mt-2.5 flex flex-wrap gap-2">
      {photos.map((p) => (
        <a
          key={p.id}
          href={`/photos/${p.id}/download`}
          target="_blank"
          rel="noreferrer"
          className="relative block overflow-hidden rounded-lg border"
          style={{ borderColor: C.line }}
          aria-label={`Фото ${p.id}`}
        >
          <PhotoThumb id={p.id} className="size-20 object-cover transition-transform hover:scale-105 sm:size-24" />
          {p.status !== 'done' && (
            <span
              className="absolute right-1 bottom-1 rounded px-1.5 py-0.5 text-[9.5px] font-extrabold text-white"
              style={{ background: p.status === 'failed' ? C.err : C.warn }}
            >
              {p.status === 'failed' ? 'сбой' : '↑'}
            </span>
          )}
        </a>
      ))}
    </div>
  )
}
