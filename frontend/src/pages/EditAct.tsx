import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { AnimatePresence, motion } from 'framer-motion'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type DefectTemplate, type EditRoomData, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'

/* Форма редактирования акта (часть 1: шапка + помещения + дефекты).
   Сохранение собирает те же поля, что старая HTML-форма, и шлёт их
   в проверенный POST /inspections/:id/edit. Фото и план — часть 2. */

const SECTIONS = [
  ['window', 'Окна и откосы'],
  ['ceiling', 'Потолок'],
  ['wall', 'Стены'],
  ['floor', 'Пол'],
  ['door', 'Двери'],
  ['plumbing', 'Сантехника'],
] as const

const MEASURES: [keyof RoomForm['m'], string][] = [
  ['length', 'Длина, м'], ['width', 'Ширина, м'], ['height', 'Высота, м'],
]

interface RoomForm {
  key: number
  name: string
  m: { length: string; width: string; height: string; dh: string; dw: string;
       w1h: string; w1w: string; w2h: string; w2w: string; w3h: string; w3w: string;
       w4h: string; w4w: string; w5h: string; w5w: string }
  windowType: string
  wallTypes: string[]
  simple: Record<number, string>
  walls: Record<number, [string, string, string, string]>
  notes: Record<string, string>
}

let roomKeySeq = 1

function numStr(v: number): string {
  return v ? String(v) : ''
}

function emptyRoom(): RoomForm {
  return {
    key: roomKeySeq++,
    name: '',
    m: { length: '', width: '', height: '', dh: '', dw: '',
         w1h: '', w1w: '', w2h: '', w2w: '', w3h: '', w3w: '', w4h: '', w4w: '', w5h: '', w5w: '' },
    windowType: '',
    wallTypes: [],
    simple: {},
    walls: {},
    notes: {},
  }
}

function roomFromData(r: EditRoomData): RoomForm {
  const room = emptyRoom()
  room.name = r.name
  room.m = {
    length: numStr(r.length), width: numStr(r.width), height: numStr(r.height),
    dh: numStr(r.dh), dw: numStr(r.dw),
    w1h: numStr(r.w1h), w1w: numStr(r.w1w), w2h: numStr(r.w2h), w2w: numStr(r.w2w),
    w3h: numStr(r.w3h), w3w: numStr(r.w3w), w4h: numStr(r.w4h), w4w: numStr(r.w4w),
    w5h: numStr(r.w5h), w5w: numStr(r.w5w),
  }
  room.windowType = r.window_type
  room.wallTypes = r.wall_types
  for (const d of r.defects) {
    if (d.template_id == null) {
      // Запись «Прочее»: текст в notes; value — fallback для легаси-данных
      const txt = d.notes || d.value
      if (txt) room.notes[d.section] = room.notes[d.section] ? `${room.notes[d.section]}; ${txt}` : txt
    } else if (d.section === 'wall' && d.wall_number >= 1 && d.wall_number <= 4) {
      const arr = room.walls[d.template_id] ?? ['', '', '', '']
      arr[d.wall_number - 1] = d.value
      room.walls[d.template_id] = arr
    } else {
      room.simple[d.template_id] = d.value
    }
  }
  return room
}

export default function EditAct({ user }: { user: User }) {
  const { id } = useParams()
  const actId = Number(id)
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['edit-data', actId],
    queryFn: () => api.editData(actId),
    enabled: Number.isFinite(actId),
    staleTime: 0,
  })

  const [header, setHeader] = useState<Record<string, string>>({})
  const [rooms, setRooms] = useState<RoomForm[]>([])
  const [numberTaken, setNumberTaken] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    if (!data || loaded) return
    const a = data.act
    setHeader({
      act_number: a.act_number, inspection_date: a.date, inspection_time: a.time,
      address: a.address, owner_name: a.owner_name, developer_rep_name: a.developer_rep_name,
      rooms_count: numStr(a.rooms_count), floor: numStr(a.floor), total_area: numStr(a.total_area),
      temp_outside: a.temp_outside ? String(a.temp_outside) : '', temp_inside: a.temp_inside ? String(a.temp_inside) : '',
      humidity: numStr(a.humidity), electricity: a.electricity, ventilation: a.ventilation,
      general_notes: a.general_notes,
    })
    setRooms(data.rooms.length > 0 ? data.rooms.map(roomFromData) : [emptyRoom()])
    setLoaded(true)
  }, [data, loaded])

  const templatesBySection = useMemo(() => {
    const m = new Map<string, DefectTemplate[]>()
    for (const t of data?.templates ?? []) {
      if (!m.has(t.section)) m.set(t.section, [])
      m.get(t.section)!.push(t)
    }
    return m
  }, [data])

  function setH(k: string, v: string) {
    setHeader((h) => ({ ...h, [k]: v }))
  }
  function patchRoom(key: number, patch: (r: RoomForm) => RoomForm) {
    setRooms((rs) => rs.map((r) => (r.key === key ? patch({ ...r }) : r)))
  }

  async function checkNumber() {
    const v = header.act_number?.trim()
    if (!v || v === data?.act.act_number) { setNumberTaken(null); return }
    try {
      const res = await api.checkActNumber(actId, v)
      setNumberTaken(res.taken ? `Номер уже занят осмотром #${res.other_id}` : null)
    } catch { /* сеть/валидация — покажет сервер при сохранении */ }
  }

  async function save() {
    setSaving(true)
    setError('')
    const p = new URLSearchParams()
    for (const [k, v] of Object.entries(header)) p.set(k, v)
    p.set('active_rooms', String(rooms.length))
    rooms.forEach((room, idx) => {
      const i = String(idx + 1)
      p.set(`room_name_${i}`, room.name)
      const mm = room.m
      const measureKeys: [string, string][] = [
        ['room_length_', mm.length], ['room_width_', mm.width], ['room_height_', mm.height],
        ['room_w1h_', mm.w1h], ['room_w1w_', mm.w1w], ['room_w2h_', mm.w2h], ['room_w2w_', mm.w2w],
        ['room_w3h_', mm.w3h], ['room_w3w_', mm.w3w], ['room_w4h_', mm.w4h], ['room_w4w_', mm.w4w],
        ['room_w5h_', mm.w5h], ['room_w5w_', mm.w5w], ['room_dh_', mm.dh], ['room_dw_', mm.dw],
      ]
      for (const [k, v] of measureKeys) if (v) p.set(k + i, v)
      if (room.windowType) p.set(`room_window_type_${i}`, room.windowType)
      for (const wt of room.wallTypes) p.set(`room_wall_type_${wt}_${i}`, '1')
      for (const [tid, val] of Object.entries(room.simple)) if (val) p.set(`defect_${tid}_${i}`, val)
      for (const [tid, vals] of Object.entries(room.walls)) {
        vals.forEach((v, w) => { if (v) p.set(`defect_${tid}_${i}_wall${w + 1}`, v) })
      }
      for (const [sec, txt] of Object.entries(room.notes)) if (txt) p.set(`notes_${sec}_${i}`, txt)
    })
    try {
      const err = await api.saveAct(actId, p)
      if (err) { setError(err); window.scrollTo({ top: 0 }) }
      else {
        await queryClient.invalidateQueries()
        navigate(`/inspections/${actId}`)
      }
    } catch {
      setError('Не удалось сохранить — проверьте соединение и попробуйте ещё раз.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />

      <main className="mx-auto max-w-4xl px-5 pt-6 pb-32">
        <a href={`/inspections/${actId}`} className="mb-5 inline-flex items-center gap-2 text-sm font-bold hover:underline" style={{ color: C.muted }}>
          ← К просмотру акта
        </a>

        {isLoading && <div className="h-72 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />}
        {isError && (
          <div className="rounded-2xl px-5 py-4 font-semibold" style={{ background: C.errBg, color: C.err }}>
            Не удалось загрузить форму. Возможно, акт удалён или у вас нет доступа.
          </div>
        )}

        {loaded && (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
            <div className="mb-4 flex items-baseline gap-3">
              <h1 className="text-[22px] font-extrabold tracking-tight">Редактирование акта</h1>
              <span className="font-mono text-[15px] font-bold tnum" style={{ color: C.muted }}>№ {data!.act.act_number}</span>
            </div>

            {error && (
              <div className="mb-4 rounded-2xl px-5 py-3.5 text-sm font-semibold" role="alert" style={{ background: C.errBg, color: C.err }}>
                {error}
              </div>
            )}

            {/* ===== Шапка акта ===== */}
            <section className="mb-5 rounded-2xl border p-5" style={{ background: C.surface, borderColor: C.line }}>
              <h2 className="mb-4 text-[13px] font-extrabold tracking-wide uppercase" style={{ color: C.muted }}>Общие сведения</h2>
              <div className="grid gap-3.5 sm:grid-cols-2">
                <Field label="Номер акта" required error={numberTaken}>
                  <input value={header.act_number ?? ''} onChange={(e) => setH('act_number', e.target.value)} onBlur={checkNumber}
                         className={inputCls} style={inputStyle(!!numberTaken)} />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Дата осмотра">
                    <input type="date" value={header.inspection_date ?? ''} onChange={(e) => setH('inspection_date', e.target.value)} className={inputCls} style={inputStyle()} />
                  </Field>
                  <Field label="Время">
                    <input type="time" value={header.inspection_time ?? ''} onChange={(e) => setH('inspection_time', e.target.value)} className={inputCls} style={inputStyle()} />
                  </Field>
                </div>
                <Field label="Адрес объекта" wide>
                  <input value={header.address ?? ''} onChange={(e) => setH('address', e.target.value)} placeholder="г. Пермь, ул. …" className={inputCls} style={inputStyle()} />
                </Field>
                <Field label="Собственник">
                  <input value={header.owner_name ?? ''} onChange={(e) => setH('owner_name', e.target.value)} className={inputCls} style={inputStyle()} />
                </Field>
                <Field label="Представитель застройщика">
                  <input value={header.developer_rep_name ?? ''} onChange={(e) => setH('developer_rep_name', e.target.value)} className={inputCls} style={inputStyle()} />
                </Field>
                <div className="grid grid-cols-3 gap-3">
                  <Field label="Комнат"><input inputMode="numeric" value={header.rooms_count ?? ''} onChange={(e) => setH('rooms_count', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                  <Field label="Этаж"><input inputMode="numeric" value={header.floor ?? ''} onChange={(e) => setH('floor', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                  <Field label="Площадь, м²"><input inputMode="decimal" value={header.total_area ?? ''} onChange={(e) => setH('total_area', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <Field label="t° нар."><input inputMode="decimal" value={header.temp_outside ?? ''} onChange={(e) => setH('temp_outside', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                  <Field label="t° внутр."><input inputMode="decimal" value={header.temp_inside ?? ''} onChange={(e) => setH('temp_inside', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                  <Field label="Влажн., %"><input inputMode="decimal" value={header.humidity ?? ''} onChange={(e) => setH('humidity', e.target.value)} className={inputCls} style={inputStyle()} /></Field>
                </div>
                <Field label="Электричество">
                  <input value={header.electricity ?? ''} onChange={(e) => setH('electricity', e.target.value)} placeholder="подключено / нет" className={inputCls} style={inputStyle()} />
                </Field>
                <Field label="Вентиляция">
                  <input value={header.ventilation ?? ''} onChange={(e) => setH('ventilation', e.target.value)} placeholder="работает / нет" className={inputCls} style={inputStyle()} />
                </Field>
                <Field label="Общие замечания" wide>
                  <textarea value={header.general_notes ?? ''} onChange={(e) => setH('general_notes', e.target.value)} rows={2} className={inputCls} style={inputStyle()} />
                </Field>
              </div>
            </section>

            {/* ===== Помещения ===== */}
            <div className="mb-3 flex items-center gap-3">
              <h2 className="text-[13px] font-extrabold tracking-wide uppercase" style={{ color: C.muted }}>
                Помещения · {rooms.length}
              </h2>
              <div className="flex-1" />
              <button
                type="button"
                onClick={() => setRooms((rs) => [...rs, emptyRoom()])}
                className="cursor-pointer rounded-full border px-4 py-2 text-[13px] font-bold transition-colors hover:underline"
                style={{ borderColor: C.line, color: C.accentDark }}
              >
                ＋ Добавить помещение
              </button>
            </div>

            <AnimatePresence>
              {rooms.map((room, idx) => (
                <motion.div key={room.key} layout initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, scale: 0.98 }}>
                  <RoomEditor
                    room={room} index={idx + 1}
                    templatesBySection={templatesBySection}
                    onPatch={(patch) => patchRoom(room.key, patch)}
                    onRemove={rooms.length > 1 ? () => setRooms((rs) => rs.filter((r) => r.key !== room.key)) : undefined}
                  />
                </motion.div>
              ))}
            </AnimatePresence>

            {/* ===== Панель сохранения ===== */}
            <div className="fixed inset-x-0 bottom-0 z-30 border-t px-5 py-3 backdrop-blur-md" style={{ background: 'rgba(248,246,241,.9)', borderColor: C.line }}>
              <div className="mx-auto flex max-w-4xl items-center gap-3">
                <span className="hidden text-[12.5px] sm:block" style={{ color: C.faint }}>
                  Фото добавляются после сохранения — на странице помещений
                </span>
                <div className="flex-1" />
                <a href={`/inspections/${actId}`} className="rounded-full border px-5 py-2.5 text-[13.5px] font-bold" style={{ borderColor: C.line, color: C.ink }}>
                  Отмена
                </a>
                <motion.button
                  whileTap={{ scale: 0.97 }}
                  onClick={save}
                  disabled={saving || !!numberTaken}
                  className="cursor-pointer rounded-full px-6 py-2.5 text-[13.5px] font-extrabold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
                  style={{ background: C.accent }}
                >
                  {saving ? 'Сохраняем…' : 'Сохранить акт'}
                </motion.button>
              </div>
            </div>
          </motion.div>
        )}
      </main>
    </div>
  )
}

/* ===== Помещение ===== */

function RoomEditor({ room, index, templatesBySection, onPatch, onRemove }: {
  room: RoomForm
  index: number
  templatesBySection: Map<string, DefectTemplate[]>
  onPatch: (patch: (r: RoomForm) => RoomForm) => void
  onRemove?: () => void
}) {
  const [open, setOpen] = useState(index === 1)
  const defectCount =
    Object.values(room.simple).filter(Boolean).length +
    Object.values(room.walls).flat().filter(Boolean).length +
    Object.values(room.notes).filter(Boolean).length

  return (
    <section className="mb-3 overflow-hidden rounded-2xl border" style={{ background: C.surface, borderColor: C.line }}>
      {/* Заголовок-аккордеон */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="flex w-full cursor-pointer items-center gap-3 px-5 py-4 text-left"
      >
        <span className="grid size-7 place-items-center rounded-lg text-[13px] font-extrabold text-white tnum" style={{ background: C.accent }}>
          {index}
        </span>
        <span className="text-[15.5px] font-extrabold">{room.name || `Помещение ${index}`}</span>
        <span className="text-[12.5px]" style={{ color: C.faint }}>
          {defectCount > 0 ? `${defectCount} дефектов` : 'без дефектов'}
        </span>
        <span className="ml-auto text-[13px]" style={{ color: C.faint }}>{open ? '▴' : '▾'}</span>
      </button>

      {open && (
        <div className="border-t px-5 pt-4 pb-5" style={{ borderColor: C.line }}>
          <div className="mb-4 grid gap-3.5 sm:grid-cols-[1fr_auto]">
            <Field label="Название помещения">
              <input value={room.name} onChange={(e) => onPatch((r) => ({ ...r, name: e.target.value }))}
                     placeholder="Гостиная, Кухня…" className={inputCls} style={inputStyle()} />
            </Field>
            {onRemove && (
              <button type="button" onClick={onRemove}
                      className="cursor-pointer self-end rounded-full border px-4 py-2.5 text-[13px] font-bold transition-colors"
                      style={{ borderColor: C.line, color: C.err }}>
                Удалить
              </button>
            )}
          </div>

          {/* Замеры */}
          <div className="mb-4 grid grid-cols-3 gap-3">
            {MEASURES.map(([k, label]) => (
              <Field key={k} label={label}>
                <input inputMode="decimal" value={room.m[k]}
                       onChange={(e) => onPatch((r) => ({ ...r, m: { ...r.m, [k]: e.target.value } }))}
                       className={inputCls} style={inputStyle()} />
              </Field>
            ))}
          </div>

          <details className="mb-4 rounded-xl border px-4 py-3" style={{ borderColor: C.line }}>
            <summary className="cursor-pointer text-[13px] font-bold" style={{ color: C.muted }}>
              Окна и дверь (размеры откосов)
            </summary>
            <div className="mt-3 grid gap-3">
              {[1, 2, 3, 4, 5].map((w) => (
                <div key={w} className="grid grid-cols-[70px_1fr_1fr] items-center gap-3">
                  <span className="text-[12.5px] font-bold" style={{ color: C.muted }}>Окно {w}</span>
                  <input inputMode="decimal" placeholder="Высота" value={room.m[`w${w}h` as keyof RoomForm['m']]}
                         onChange={(e) => onPatch((r) => ({ ...r, m: { ...r.m, [`w${w}h`]: e.target.value } }))}
                         className={inputCls} style={inputStyle()} />
                  <input inputMode="decimal" placeholder="Ширина" value={room.m[`w${w}w` as keyof RoomForm['m']]}
                         onChange={(e) => onPatch((r) => ({ ...r, m: { ...r.m, [`w${w}w`]: e.target.value } }))}
                         className={inputCls} style={inputStyle()} />
                </div>
              ))}
              <div className="grid grid-cols-[70px_1fr_1fr] items-center gap-3">
                <span className="text-[12.5px] font-bold" style={{ color: C.muted }}>Дверь</span>
                <input inputMode="decimal" placeholder="Высота" value={room.m.dh}
                       onChange={(e) => onPatch((r) => ({ ...r, m: { ...r.m, dh: e.target.value } }))}
                       className={inputCls} style={inputStyle()} />
                <input inputMode="decimal" placeholder="Ширина" value={room.m.dw}
                       onChange={(e) => onPatch((r) => ({ ...r, m: { ...r.m, dw: e.target.value } }))}
                       className={inputCls} style={inputStyle()} />
              </div>
            </div>
          </details>

          {/* Типы отделки */}
          <div className="mb-5 flex flex-wrap items-center gap-4">
            <label className="flex items-center gap-2 text-[13px] font-semibold" style={{ color: C.muted }}>
              Окна:
              <select value={room.windowType} onChange={(e) => onPatch((r) => ({ ...r, windowType: e.target.value }))}
                      className={inputCls} style={inputStyle()}>
                <option value="">—</option>
                <option value="pvc">ПВХ</option>
                <option value="al">Алюминий</option>
                <option value="wood">Дерево</option>
              </select>
            </label>
            <div className="flex items-center gap-3 text-[13px] font-semibold" style={{ color: C.muted }}>
              Стены:
              {([['paint', 'окраска'], ['tile', 'плитка'], ['gkl', 'ГКЛ']] as const).map(([val, label]) => (
                <label key={val} className="flex cursor-pointer items-center gap-1.5">
                  <input
                    type="checkbox"
                    checked={room.wallTypes.includes(val)}
                    onChange={(e) =>
                      onPatch((r) => ({
                        ...r,
                        wallTypes: e.target.checked ? [...r.wallTypes, val] : r.wallTypes.filter((t) => t !== val),
                      }))
                    }
                  />
                  {label}
                </label>
              ))}
            </div>
          </div>

          {/* Дефекты по разделам */}
          {SECTIONS.map(([sec, secName]) => {
            const tpls = templatesBySection.get(sec) ?? []
            return (
              <details key={sec} className="mb-2 rounded-xl border px-4 py-3" style={{ borderColor: C.line }}>
                <summary className="cursor-pointer text-[13px] font-bold">
                  <span style={{ color: C.ink }}>{secName}</span>
                  <SectionBadge room={room} section={sec} tpls={tpls} />
                </summary>
                <div className="mt-3 grid gap-2.5">
                  {sec === 'wall'
                    ? tpls.map((t) => (
                        <div key={t.id} className="grid gap-1.5">
                          <span className="text-[12.5px] font-semibold" style={{ color: C.muted }}>
                            {t.name}{t.unit && `, ${t.unit}`}{t.threshold && ` (норма: ${t.threshold})`}
                          </span>
                          <div className="grid grid-cols-4 gap-2">
                            {[0, 1, 2, 3].map((w) => (
                              <input
                                key={w}
                                placeholder={`ст. ${w + 1}`}
                                value={room.walls[t.id]?.[w] ?? ''}
                                onChange={(e) =>
                                  onPatch((r) => {
                                    const arr = [...(r.walls[t.id] ?? ['', '', '', ''])] as [string, string, string, string]
                                    arr[w] = e.target.value
                                    return { ...r, walls: { ...r.walls, [t.id]: arr } }
                                  })
                                }
                                className={inputCls} style={inputStyle()}
                              />
                            ))}
                          </div>
                        </div>
                      ))
                    : tpls.map((t) => (
                        <label key={t.id} className="grid grid-cols-[1fr_130px] items-center gap-3">
                          <span className="text-[13px]" style={{ color: C.ink }}>
                            {t.name}
                            {(t.unit || t.threshold) && (
                              <span style={{ color: C.faint }}>{t.unit && ` · ${t.unit}`}{t.threshold && ` · норма ${t.threshold}`}</span>
                            )}
                          </span>
                          <input
                            placeholder="значение"
                            value={room.simple[t.id] ?? ''}
                            onChange={(e) => onPatch((r) => ({ ...r, simple: { ...r.simple, [t.id]: e.target.value } }))}
                            className={inputCls} style={inputStyle()}
                          />
                        </label>
                      ))}
                  <label className="grid gap-1.5">
                    <span className="text-[12.5px] font-semibold" style={{ color: C.muted }}>Прочее (свободный текст)</span>
                    <textarea
                      rows={2}
                      value={room.notes[sec] ?? ''}
                      onChange={(e) => onPatch((r) => ({ ...r, notes: { ...r.notes, [sec]: e.target.value } }))}
                      className={inputCls} style={inputStyle()}
                    />
                  </label>
                </div>
              </details>
            )
          })}
        </div>
      )}
    </section>
  )
}

function SectionBadge({ room, section, tpls }: { room: RoomForm; section: string; tpls: DefectTemplate[] }) {
  const ids = new Set(tpls.map((t) => t.id))
  let n = 0
  for (const [tid, v] of Object.entries(room.simple)) if (v && ids.has(Number(tid))) n++
  for (const [tid, vals] of Object.entries(room.walls)) if (ids.has(Number(tid))) n += vals.filter(Boolean).length
  if (room.notes[section]) n++
  if (n === 0) return null
  return (
    <span className="ml-2 rounded-full px-2 py-0.5 text-[11px] font-extrabold" style={{ background: C.accentSoft, color: C.accentDark }}>
      {n}
    </span>
  )
}

/* ===== Мелочи ===== */

const inputCls = 'w-full rounded-[10px] border px-3 py-2.5 text-[14px] outline-none focus:ring-2'

function inputStyle(invalid = false): React.CSSProperties {
  return {
    background: C.surface,
    borderColor: invalid ? C.err : C.line,
    color: C.ink,
    ['--tw-ring-color' as string]: C.accentSoft,
  }
}

function Field({ label, required, wide, error, children }: {
  label: string
  required?: boolean
  wide?: boolean
  error?: string | null
  children: React.ReactNode
}) {
  return (
    <label className={`grid content-start gap-1.5 ${wide ? 'sm:col-span-2' : ''}`}>
      <span className="text-[12px] font-bold tracking-wide uppercase" style={{ color: C.muted }}>
        {label}{required && <span style={{ color: C.err }}> *</span>}
      </span>
      {children}
      {error && <span className="text-[12px] font-semibold" style={{ color: C.err }}>{error}</span>}
    </label>
  )
}
