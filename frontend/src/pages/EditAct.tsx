import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type User } from '../lib/api'
import { C } from '../lib/palette'
import { isActive, useUploadQueue } from '../lib/uploadQueue'
import Header from '../components/Header'
import PlanCard from './edit/PlanCard'
import RoomCard from './edit/RoomCard'
import { buildParams, bindsFrom, emptyRoom, numStr, roomFromData, type RoomForm } from './edit/form'
import { Button, Card, CheckIcon, Chip, Collapse, Field, PlusIcon, TextArea, TextInput } from './edit/ui'

/* Редактор акта в структуре редактора осмотров CRM: основные данные,
   свёртка параметров объекта, план, помещения аккордеоном с дефектами
   из справочника, липкая панель сохранения. Сохранение — тот же
   POST /inspections/:id/edit с автосейвом. */

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
  const [expanded, setExpanded] = useState<number[]>([])
  const [paramsOpen, setParamsOpen] = useState(false)
  const [planUrl, setPlanUrl] = useState('')
  const [numberTaken, setNumberTaken] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [lastSaved, setLastSaved] = useState('')
  const queueItems = useUploadQueue()
  const uploadsActive = queueItems.filter((i) => i.actId === actId && isActive(i)).length
  // Счётчик правок: автосейв снимает dirty только если за время POST
  // не появилось новых изменений
  const changeSeq = useRef(0)

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
    const rs = data.rooms.length > 0 ? data.rooms.map(roomFromData) : [emptyRoom()]
    setRooms(rs)
    setExpanded(rs.length ? [rs[0].key] : [])
    setParamsOpen(!a.total_area)
    setPlanUrl(a.plan_image)
    setLoaded(true)
  }, [data, loaded])

  function markDirty() {
    changeSeq.current++
    setDirty(true)
  }
  function setH(k: string, v: string) {
    markDirty()
    setHeader((h) => ({ ...h, [k]: v }))
  }
  function patchRoom(key: number, patch: (r: RoomForm) => RoomForm) {
    markDirty()
    setRooms((rs) => rs.map((r) => (r.key === key ? patch({ ...r }) : r)))
  }
  // Фото живут на сервере сразу — их изменения форму «грязной» не делают
  function patchRoomSilent(key: number, patch: (r: RoomForm) => RoomForm) {
    setRooms((rs) => rs.map((r) => (r.key === key ? patch({ ...r }) : r)))
  }
  function addRoom() {
    const room = emptyRoom()
    markDirty()
    setRooms((rs) => [...rs, room])
    setExpanded((e) => [...e, room.key])
  }
  const toggleRoom = (key: number) => setExpanded((e) => (e.includes(key) ? e.filter((k) => k !== key) : [...e, key]))

  async function checkNumber() {
    const v = header.act_number?.trim()
    if (!v || v === data?.act.act_number) { setNumberTaken(null); return }
    try {
      const res = await api.checkActNumber(actId, v)
      setNumberTaken(res.taken ? `Номер уже занят осмотром #${res.other_id}` : null)
    } catch { /* сеть/валидация — покажет сервер при сохранении */ }
  }

  // После сохранения дефекты пересозданы с новыми id — перечитываем
  // привязки фото, не трогая введённые пользователем значения
  async function refreshBinds() {
    const d = await api.editData(actId)
    setPlanUrl(d.act.plan_image)
    setRooms((rs) =>
      rs.map((room, idx) => {
        const serverRoom = d.rooms.find((r) => r.number === idx + 1)
        return serverRoom ? { ...room, binds: bindsFrom(serverRoom) } : room
      }),
    )
  }

  async function doSave(auto: boolean) {
    const seq = changeSeq.current
    setSaving(true)
    if (!auto) setError('')
    try {
      const err = await api.saveAct(actId, buildParams(header, rooms))
      if (err) {
        setError(err)
        if (!auto) window.scrollTo({ top: 0 })
        return
      }
      if (changeSeq.current === seq) setDirty(false)
      setLastSaved(new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }))
      if (auto) {
        await refreshBinds()
      } else {
        await queryClient.invalidateQueries()
        navigate(`/inspections/${actId}`)
      }
    } catch {
      if (!auto) setError('Не удалось сохранить — проверьте соединение и попробуйте ещё раз.')
    } finally {
      setSaving(false)
    }
  }

  // Автосейв: 2.5 сек тишины после правок; пауза, пока грузятся фото
  // или занят номер акта
  useEffect(() => {
    if (!dirty || !loaded || saving || uploadsActive > 0 || numberTaken) return
    const t = setTimeout(() => doSave(true), 2500)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dirty, loaded, saving, uploadsActive, numberTaken, header, rooms])

  useEffect(() => {
    if (!dirty) return
    const h = (e: BeforeUnloadEvent) => { e.preventDefault() }
    window.addEventListener('beforeunload', h)
    return () => window.removeEventListener('beforeunload', h)
  }, [dirty])

  const statusText = saving
    ? 'Сохраняем…'
    : uploadsActive > 0
      ? `Отправляем фото: ${uploadsActive}`
      : dirty
        ? 'Есть несохранённые изменения'
        : lastSaved
          ? `Сохранено ${lastSaved}`
          : ''

  const allPhotos = rooms.flatMap((r) => Object.values(r.binds).flatMap((b) => b.photos))
  const cloudFailed = allPhotos.filter((p) => p.status === 'failed').length
  const cloudPending = allPhotos.filter((p) => p.status === 'pending' || p.status === 'uploading').length
  const roomsN = rooms.length
  const paramsSummary = [
    header.total_area && `${header.total_area} м²`,
    header.floor && `${header.floor} этаж`,
    header.owner_name,
  ].filter(Boolean).join(' · ')

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />

      <main className="mx-auto flex max-w-4xl flex-col gap-4 px-4 pt-4 pb-28 sm:px-5 sm:pt-5">
        {isLoading && <div className="h-72 animate-pulse rounded-xl motion-reduce:animate-none" style={{ background: C.track }} />}
        {isError && (
          <div className="rounded-xl px-4 py-3 text-[13px] font-semibold" style={{ background: C.errBg, color: C.err }}>
            Не удалось загрузить форму. Возможно, акт удалён или у вас нет доступа.
          </div>
        )}

        {loaded && data && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="mr-1 text-[20px] font-semibold tracking-tight">Осмотр №{data.act.act_number}</h1>
              <Chip variant={data.act.status === 'completed' ? 'success' : 'info'}>
                {data.act.status === 'completed' ? 'Завершён' : 'В работе'}
              </Chip>
              {cloudFailed > 0 ? (
                <Chip variant="danger">сбой выгрузки: {cloudFailed}</Chip>
              ) : cloudPending > 0 ? (
                <Chip variant="warn">фото в очереди: {cloudPending}</Chip>
              ) : allPhotos.length > 0 ? (
                <Chip variant="success">фото в облаке ✓</Chip>
              ) : null}
            </div>

            <div>
              <Link to={`/inspections/${actId}`} className="inline-flex items-center gap-1.5 text-[13px] font-semibold" style={{ color: C.muted }}>
                ← К просмотру акта
              </Link>
            </div>

            {error && (
              <div className="rounded-xl px-4 py-3 text-[13px] font-semibold" role="alert" style={{ background: C.errBg, color: C.err }}>
                {error}
              </div>
            )}

            <Card title="Основные данные">
              <div className="grid gap-3 sm:grid-cols-2">
                <Field label="Номер акта" error={numberTaken}>
                  <TextInput value={header.act_number ?? ''} invalid={!!numberTaken} onChange={(e) => setH('act_number', e.target.value)} onBlur={checkNumber} className="tnum" />
                </Field>
                <div className="grid grid-cols-2 gap-3">
                  <Field label="Дата осмотра">
                    <TextInput type="date" value={header.inspection_date ?? ''} onChange={(e) => setH('inspection_date', e.target.value)} />
                  </Field>
                  <Field label="Время">
                    <TextInput type="time" value={header.inspection_time ?? ''} onChange={(e) => setH('inspection_time', e.target.value)} />
                  </Field>
                </div>
                <Field label="Адрес объекта" className="sm:col-span-2">
                  <TextInput value={header.address ?? ''} placeholder="г. Пермь, ул. …" onChange={(e) => setH('address', e.target.value)} />
                </Field>
              </div>
            </Card>

            <Collapse
              open={paramsOpen}
              onToggle={() => setParamsOpen((v) => !v)}
              header={
                <span className="flex flex-wrap items-center gap-2.5">
                  <span className="text-[14px] font-semibold">Параметры объекта</span>
                  {!paramsOpen && paramsSummary && <span className="text-[12px]" style={{ color: C.muted }}>{paramsSummary}</span>}
                </span>
              }
            >
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-6">
                <Field label="Этаж"><TextInput inputMode="numeric" value={header.floor ?? ''} onChange={(e) => setH('floor', e.target.value)} /></Field>
                <Field label="Помещений"><TextInput inputMode="numeric" value={header.rooms_count ?? ''} onChange={(e) => setH('rooms_count', e.target.value)} /></Field>
                <Field label="Площадь, м²"><TextInput inputMode="decimal" value={header.total_area ?? ''} onChange={(e) => setH('total_area', e.target.value)} /></Field>
                <Field label="t° снаружи"><TextInput inputMode="decimal" value={header.temp_outside ?? ''} onChange={(e) => setH('temp_outside', e.target.value)} /></Field>
                <Field label="t° внутри"><TextInput inputMode="decimal" value={header.temp_inside ?? ''} onChange={(e) => setH('temp_inside', e.target.value)} /></Field>
                <Field label="Влажность, %"><TextInput inputMode="decimal" value={header.humidity ?? ''} onChange={(e) => setH('humidity', e.target.value)} /></Field>
                <Field label="ФИО собственника" className="col-span-2 sm:col-span-3"><TextInput value={header.owner_name ?? ''} onChange={(e) => setH('owner_name', e.target.value)} /></Field>
                <Field label="Представитель застройщика" className="col-span-2 sm:col-span-3"><TextInput value={header.developer_rep_name ?? ''} onChange={(e) => setH('developer_rep_name', e.target.value)} /></Field>
                <Field label="Электрика" className="col-span-2 sm:col-span-3"><TextInput value={header.electricity ?? ''} placeholder="подключено / нет" onChange={(e) => setH('electricity', e.target.value)} /></Field>
                <Field label="Вентиляция" className="col-span-2 sm:col-span-3"><TextInput value={header.ventilation ?? ''} placeholder="работает / нет" onChange={(e) => setH('ventilation', e.target.value)} /></Field>
                <Field label="Общие замечания" className="col-span-2 sm:col-span-6"><TextArea rows={2} value={header.general_notes ?? ''} onChange={(e) => setH('general_notes', e.target.value)} /></Field>
              </div>
            </Collapse>

            <PlanCard actId={actId} planUrl={planUrl} onUploaded={() => api.editData(actId).then((d) => setPlanUrl(d.act.plan_image))} />

            <Card
              title={<span className="block truncate">Помещения и дефекты{roomsN > 0 && <span style={{ color: C.muted }}> · {roomsN}</span>}</span>}
              extra={
                <Button variant="primary" icon={<PlusIcon />} onClick={addRoom}>
                  <span className="hidden sm:inline">Добавить помещение</span>
                  <span className="sm:hidden">Помещение</span>
                </Button>
              }
            >
              <div className="flex flex-col gap-2.5">
                {roomsN === 0 && <span className="text-[13px]" style={{ color: C.faint }}>Помещений пока нет — добавьте первое.</span>}
                {rooms.map((room, idx) => (
                  <RoomCard
                    key={room.key}
                    room={room}
                    index={idx + 1}
                    templates={data.templates}
                    actId={actId}
                    open={expanded.includes(room.key)}
                    onToggle={() => toggleRoom(room.key)}
                    onPatch={(patch) => patchRoom(room.key, patch)}
                    onPatchSilent={(patch) => patchRoomSilent(room.key, patch)}
                    onRemove={rooms.length > 1 ? () => { markDirty(); setRooms((rs) => rs.filter((r) => r.key !== room.key)) } : undefined}
                  />
                ))}
                {roomsN > 0 && (
                  <Button variant="dashed" block icon={<PlusIcon />} onClick={addRoom}>
                    Добавить помещение
                  </Button>
                )}
              </div>
            </Card>

            <div className="fixed inset-x-0 bottom-0 z-30 border-t px-4 py-3 backdrop-blur-md sm:px-5" style={{ background: 'rgba(255,255,255,.88)', borderColor: C.line, paddingBottom: 'max(12px, env(safe-area-inset-bottom))' }}>
              <div className="mx-auto flex max-w-4xl items-center gap-3">
                <span className="min-w-0 flex-1 truncate text-[13px]" style={{ color: error ? C.err : C.muted }} aria-live="polite">
                  {statusText}
                </span>
                <Button variant="primary" icon={<CheckIcon />} disabled={saving || !!numberTaken} onClick={() => doSave(false)}>
                  {saving ? 'Сохраняем…' : 'Сохранить и выйти'}
                </Button>
              </div>
            </div>
          </>
        )}
      </main>
    </div>
  )
}
