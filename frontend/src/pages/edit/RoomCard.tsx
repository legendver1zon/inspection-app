import { useMemo, useState, type ReactNode } from 'react'
import type { DefectTemplate } from '../../lib/api'
import { C } from '../../lib/palette'
import DefectPicker from './DefectPicker'
import PhotoDock, { type BindRef } from './PhotoDock'
import {
  SECTIONS, SECTION_LABEL, WALLS, WALL_TYPES, WINDOW_TYPES,
  defectCount, measured, photoCount, plural, windowsCount,
  type DefectBind, type Measures, type RoomForm,
} from './form'
import { Button, Chip, Collapse, Field, PlusIcon, Select, TextArea, TextInput, TrashIcon } from './ui'

type Patch = (fn: (r: RoomForm) => RoomForm) => void

export default function RoomCard({ room, index, templates, actId, open, onToggle, onPatch, onPatchSilent, onRemove }: {
  room: RoomForm
  index: number
  templates: DefectTemplate[]
  actId: number
  open: boolean
  onToggle: () => void
  onPatch: Patch
  onPatchSilent: Patch
  onRemove?: () => void
}) {
  const nDef = defectCount(room)
  const nPhoto = photoCount(room)
  return (
    <Collapse
      open={open}
      onToggle={onToggle}
      header={
        <span className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="grid size-[26px] flex-none place-items-center rounded-lg text-[12px] font-semibold tnum" style={{ background: C.accentSoft, color: C.accentDark }}>
            {index}
          </span>
          <span className="text-[14px] font-semibold">{room.name || 'Помещение'}</span>
          {nDef > 0 && <Chip variant="info">{nDef} {plural(nDef, 'дефект', 'дефекта', 'дефектов')}</Chip>}
          {nPhoto > 0 && <Chip>{nPhoto} фото</Chip>}
        </span>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label="Название помещения">
          <TextInput value={room.name} placeholder="Кухня, гостиная…" onChange={(e) => onPatch((r) => ({ ...r, name: e.target.value }))} />
        </Field>
        <RoomParams room={room} onPatch={onPatch} />
        <span className="text-[14px] font-semibold">Дефекты{nDef > 0 && ` · ${nDef}`}</span>
        <Defects room={room} templates={templates} actId={actId} onPatch={onPatch} onPatchSilent={onPatchSilent} />
        {onRemove && (
          <div className="flex justify-end">
            <Button variant="danger" icon={<TrashIcon />} onClick={() => { if (window.confirm('Удалить помещение вместе с дефектами и фото?')) onRemove() }}>
              Удалить помещение
            </Button>
          </div>
        )}
      </div>
    </Collapse>
  )
}

/* ===== Параметры помещения ===== */

function RoomParams({ room, onPatch }: { room: RoomForm; onPatch: Patch }) {
  const [open, setOpen] = useState(false)
  const set = (patch: Partial<RoomForm>) => onPatch((r) => ({ ...r, ...patch }))
  const setM = (k: keyof Measures, v: string) => onPatch((r) => ({ ...r, m: { ...r.m, [k]: v } }))
  const winN = windowsCount(room)
  const [visibleWindows, setVisibleWindows] = useState(() => Math.max(winN, 0))
  const shown = Math.max(visibleWindows, winN)

  const summary = measured(room)
    ? `${[room.m.length, room.m.width, room.m.height].join('×')} м${winN > 0 ? ` · ${winN} ${plural(winN, 'окно', 'окна', 'окон')}` : ''}`
    : 'замеры не заполнены'

  return (
    <Collapse
      open={open}
      onToggle={() => setOpen((v) => !v)}
      header={
        <span className="flex flex-wrap items-center gap-2.5">
          <span className="text-[14px] font-semibold">Параметры помещения</span>
          {!open && (
            <span className="text-[12px] tnum" style={{ color: measured(room) ? C.muted : C.warn, fontWeight: measured(room) ? 400 : 500 }}>
              {summary}
            </span>
          )}
        </span>
      }
    >
      <div className="flex flex-col gap-3">
        <div className="grid grid-cols-3 gap-3">
          <Field label="Длина, м"><TextInput inputMode="decimal" value={room.m.length} onChange={(e) => setM('length', e.target.value)} /></Field>
          <Field label="Ширина, м"><TextInput inputMode="decimal" value={room.m.width} onChange={(e) => setM('width', e.target.value)} /></Field>
          <Field label="Высота, м"><TextInput inputMode="decimal" value={room.m.height} onChange={(e) => setM('height', e.target.value)} /></Field>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Дверь высота, м"><TextInput inputMode="decimal" value={room.m.dh} onChange={(e) => setM('dh', e.target.value)} /></Field>
          <Field label="Дверь ширина, м"><TextInput inputMode="decimal" value={room.m.dw} onChange={(e) => setM('dw', e.target.value)} /></Field>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Окно (тип)">
            <Select value={room.windowType} onChange={(e) => set({ windowType: e.target.value })}>
              {WINDOW_TYPES.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </Select>
          </Field>
          <Field label="Стены">
            <div className="grid grid-cols-3 gap-1.5">
              {WALL_TYPES.map(([v, l]) => {
                const on = room.wallTypes.includes(v)
                return (
                  <Button
                    key={v}
                    variant={on ? 'ghost-active' : 'default'}
                    aria-pressed={on}
                    className="min-w-0 px-1 text-[13px]"
                    onClick={() => set({ wallTypes: on ? room.wallTypes.filter((t) => t !== v) : [...room.wallTypes, v] })}
                  >
                    {l}
                  </Button>
                )
              })}
            </div>
          </Field>
        </div>

        <div className="mt-1 flex items-center justify-between">
          <span className="text-[14px] font-semibold">Окна{winN > 0 && ` · ${winN}`}</span>
          {shown < 5 && (
            <Button icon={<PlusIcon />} onClick={() => setVisibleWindows(shown + 1)}>
              Добавить окно
            </Button>
          )}
        </div>
        {Array.from({ length: shown }, (_, i) => i + 1).map((w) => {
          const hk = `w${w}h` as keyof Measures
          const wk = `w${w}w` as keyof Measures
          return (
            <div key={w} className="grid grid-cols-[52px_1fr_1fr_auto] items-end gap-2">
              <span className="pb-3 text-[12px] font-semibold tnum" style={{ color: C.muted }}>Окно {w}</span>
              <Field label="Высота, м"><TextInput inputMode="decimal" value={room.m[hk]} onChange={(e) => setM(hk, e.target.value)} /></Field>
              <Field label="Ширина, м"><TextInput inputMode="decimal" value={room.m[wk]} onChange={(e) => setM(wk, e.target.value)} /></Field>
              <Button
                variant="danger-text"
                className="px-2"
                aria-label={`Убрать окно ${w}`}
                onClick={() => {
                  onPatch((r) => ({ ...r, m: { ...r.m, [hk]: '', [wk]: '' } }))
                  setVisibleWindows((n) => Math.max(0, n - 1))
                }}
              >
                <TrashIcon />
              </Button>
            </div>
          )
        })}
        {shown === 0 && <span className="text-[12px]" style={{ color: C.faint }}>Окон не зафиксировано</span>}
      </div>
    </Collapse>
  )
}

/* ===== Дефекты помещения ===== */

function Defects({ room, templates, actId, onPatch, onPatchSilent }: {
  room: RoomForm
  templates: DefectTemplate[]
  actId: number
  onPatch: Patch
  onPatchSilent: Patch
}) {
  const [pickerOpen, setPickerOpen] = useState(false)
  const tplById = useMemo(() => new Map(templates.map((t) => [t.id, t])), [templates])
  const sectionOrder = useMemo(() => new Map(SECTIONS.map(([s], i) => [s, i])), [])

  const cards = useMemo(() => {
    const order = (key: string) => {
      if (key.startsWith('n')) return [sectionOrder.get(key.slice(1)) ?? 99, 1e9]
      const t = tplById.get(Number(key.slice(1)))
      return [sectionOrder.get(t?.section ?? '') ?? 99, t?.id ?? 0]
    }
    return [...room.picked].sort((a, b) => {
      const [sa, ia] = order(a)
      const [sb, ib] = order(b)
      return sa - sb || ia - ib
    })
  }, [room.picked, tplById, sectionOrder])

  const setBind = (key: string, b: DefectBind) => onPatchSilent((r) => ({ ...r, binds: { ...r.binds, [key]: b } }))

  const pick = (t: DefectTemplate) => {
    onPatch((r) => (r.picked.includes(`s${t.id}`) ? r : { ...r, picked: [...r.picked, `s${t.id}`] }))
  }
  const addCustom = (section: string) => {
    onPatch((r) => (r.picked.includes(`n${section}`) ? r : { ...r, picked: [...r.picked, `n${section}`] }))
    setPickerOpen(false)
  }

  // Стены: чипы отмечают стены, значение общее на все отмеченные.
  const toggleWall = (tplId: number, w: number) => {
    const on = room.wallsOn[tplId]?.[w] ?? false
    if (on) {
      const photos = room.binds[`w${tplId}_${w}`]?.photos.length ?? 0
      if (photos > 0 && !window.confirm(`Убрать стену ${w + 1}? Фото по ней уйдут в архив.`)) return
    }
    onPatch((r) => {
      const wallsOn = [...(r.wallsOn[tplId] ?? [false, false, false, false])] as RoomForm['wallsOn'][number]
      const walls = [...(r.walls[tplId] ?? ['', '', '', ''])] as RoomForm['walls'][number]
      const shared = walls.find((v, i) => wallsOn[i] && v.trim()) ?? ''
      wallsOn[w] = !on
      walls[w] = on ? '' : shared
      const picked = r.picked.includes(`w${tplId}`) ? r.picked : [...r.picked, `w${tplId}`]
      return { ...r, wallsOn: { ...r.wallsOn, [tplId]: wallsOn }, walls: { ...r.walls, [tplId]: walls }, picked }
    })
  }
  const setWallValue = (tplId: number, value: string) => {
    onPatch((r) => {
      const wallsOn = r.wallsOn[tplId] ?? [false, false, false, false]
      const walls = WALLS.map((w) => (wallsOn[w] ? value : '')) as RoomForm['walls'][number]
      return { ...r, walls: { ...r.walls, [tplId]: walls } }
    })
  }

  const removeCard = (key: string) => {
    const bindKeys = key.startsWith('w') ? WALLS.map((w) => `${key}_${w}`) : [key]
    const photos = bindKeys.reduce((s, k) => s + (room.binds[k]?.photos.length ?? 0), 0)
    if (!window.confirm(photos > 0 ? 'Удалить дефект вместе с фото?' : 'Удалить дефект?')) return
    onPatch((r) => {
      const next = { ...r, picked: r.picked.filter((k) => k !== key) }
      if (key.startsWith('s')) {
        const { [Number(key.slice(1))]: _, ...simple } = r.simple
        next.simple = simple
      } else if (key.startsWith('w')) {
        const id = Number(key.slice(1))
        const { [id]: _a, ...walls } = r.walls
        const { [id]: _b, ...wallsOn } = r.wallsOn
        next.walls = walls
        next.wallsOn = wallsOn
      } else {
        const { [key.slice(1)]: _, ...notes } = r.notes
        next.notes = notes
      }
      return next
    })
  }

  const valueHint = (hasValue: boolean, bound: boolean) => {
    if (!hasValue) return 'Без значения дефект не попадёт в акт'
    if (!bound) return 'Фото можно будет добавить после сохранения'
    return undefined
  }

  return (
    <div className="flex flex-col gap-2.5">
      {cards.length === 0 && (
        <span className="text-[13px]" style={{ color: C.faint }}>Дефектов пока нет — добавьте из справочника.</span>
      )}

      {cards.map((key) => {
        if (key.startsWith('w')) {
          const id = Number(key.slice(1))
          const t = tplById.get(id)
          const on = room.wallsOn[id] ?? [false, false, false, false]
          const vals = room.walls[id] ?? ['', '', '', '']
          const active = WALLS.filter((w) => on[w])
          const shared = vals.find((v, i) => on[i] && v.trim()) ?? ''
          const differ = active.some((w) => vals[w].trim() !== shared.trim())
          const binds: BindRef[] = active.flatMap((w) => (room.binds[`${key}_${w}`] ? [{ key: `${key}_${w}`, bind: room.binds[`${key}_${w}`] }] : []))
          return (
            <DefectShell key={key} name={t?.name ?? 'Дефект из справочника'} sub={`Стены${t?.threshold ? ` · норма ${t.threshold}` : ''}`} onRemove={() => removeCard(key)}>
              <div className="grid grid-cols-4 gap-1.5" role="group" aria-label="Стены с дефектом">
                {WALLS.map((w) => (
                  <Button key={w} variant={on[w] ? 'ghost-active' : 'default'} aria-pressed={on[w]} className="min-w-0 px-1 text-[13px]" onClick={() => toggleWall(id, w)}>
                    Ст. {w + 1}
                  </Button>
                ))}
              </div>
              {active.length === 0 && <span className="text-[12px] font-medium" style={{ color: C.warn }}>Стена не указана — отметьте её выше</span>}
              <Field label={`Значение${t?.unit ? `, ${t.unit}` : ''}`} hint={valueHint(!!shared.trim(), binds.length > 0)}>
                <TextInput inputMode={t?.unit ? 'decimal' : undefined} placeholder={t?.unit || 'есть / описание'} value={shared} onChange={(e) => setWallValue(id, e.target.value)} />
              </Field>
              {differ && (
                <span className="text-[12px]" style={{ color: C.faint }}>
                  Значения по стенам различаются: {active.map((w) => `ст. ${w + 1} — ${vals[w] || '—'}`).join(', ')}. После правки станет общим.
                </span>
              )}
              <PhotoDock actId={actId} binds={binds} canUpload={binds.length > 0} onBind={setBind} />
            </DefectShell>
          )
        }

        if (key.startsWith('n')) {
          const sec = key.slice(1)
          const bind = room.binds[key]
          const text = room.notes[sec] ?? ''
          return (
            <DefectShell key={key} name="Свой дефект" chip={SECTION_LABEL[sec] ?? sec} onRemove={() => removeCard(key)}>
              <Field label="Описание" hint={valueHint(!!text.trim(), !!bind)}>
                <TextArea rows={2} placeholder="Опишите дефект" value={text} onChange={(e) => onPatch((r) => ({ ...r, notes: { ...r.notes, [sec]: e.target.value } }))} />
              </Field>
              <PhotoDock actId={actId} binds={bind ? [{ key, bind }] : []} canUpload={!!bind} onBind={setBind} />
            </DefectShell>
          )
        }

        const id = Number(key.slice(1))
        const t = tplById.get(id)
        const bind = room.binds[key]
        const value = room.simple[id] ?? ''
        return (
          <DefectShell key={key} name={t?.name ?? 'Дефект из справочника'} chip={SECTION_LABEL[t?.section ?? ''] ?? t?.section} sub={t?.threshold ? `норма ${t.threshold}` : undefined} onRemove={() => removeCard(key)}>
            <Field label={`Значение${t?.unit ? `, ${t.unit}` : ''}`} hint={valueHint(!!value.trim(), !!bind)}>
              <TextInput inputMode={t?.unit ? 'decimal' : undefined} placeholder={t?.unit || 'есть / описание'} value={value} onChange={(e) => onPatch((r) => ({ ...r, simple: { ...r.simple, [id]: e.target.value } }))} />
            </Field>
            <PhotoDock actId={actId} binds={bind ? [{ key, bind }] : []} canUpload={!!bind} onBind={setBind} />
          </DefectShell>
        )
      })}

      <Button variant="dashed" block icon={<PlusIcon />} onClick={() => setPickerOpen(true)}>
        Добавить дефект
      </Button>

      <DefectPicker
        open={pickerOpen}
        room={room}
        templates={templates}
        onPick={pick}
        onToggleWall={toggleWall}
        onAddCustom={addCustom}
        onClose={() => setPickerOpen(false)}
      />
    </div>
  )
}

function DefectShell({ name, chip, sub, onRemove, children }: {
  name: string
  chip?: string
  sub?: string
  onRemove: () => void
  children: ReactNode
}) {
  return (
    <div className="flex flex-col gap-2.5 rounded-xl border p-3" style={{ background: C.bg, borderColor: C.line }}>
      <div className="flex items-start gap-2">
        <span className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="text-[14px] leading-snug font-semibold">{name}</span>
          <span className="flex items-center gap-1.5 text-[12px]" style={{ color: C.muted }}>
            {chip && <Chip>{chip}</Chip>}
            {sub}
          </span>
        </span>
        <Button variant="danger-text" className="-mt-1 -mr-1 h-8 px-2 sm:h-8" aria-label="Удалить дефект" onClick={onRemove}>
          <TrashIcon />
        </Button>
      </div>
      {children}
    </div>
  )
}
