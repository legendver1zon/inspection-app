import { useMemo, useState } from 'react'
import type { DefectTemplate } from '../../lib/api'
import { C } from '../../lib/palette'
import { SECTIONS, SECTION_LABEL, WALLS, type RoomForm } from './form'
import { ArrowLeftIcon, Button, ChevronRightIcon, Drawer, SearchIcon, TextInput } from './ui'

// Два шага, как в CRM: область (окна, стены…) → дефект из её списка.
// В «Стенах» дефект отмечается сразу по стенам — переключателями.
export default function DefectPicker({ open, room, templates, onPick, onToggleWall, onAddCustom, onClose }: {
  open: boolean
  room: RoomForm
  templates: DefectTemplate[]
  onPick: (t: DefectTemplate) => void
  onToggleWall: (tplId: number, wall: number) => void
  onAddCustom: (section: string) => void
  onClose: () => void
}) {
  const [area, setArea] = useState<string | null>(null)
  const [query, setQuery] = useState('')

  const totalBySection = useMemo(() => {
    const m = new Map<string, number>()
    for (const t of templates) m.set(t.section, (m.get(t.section) ?? 0) + 1)
    return m
  }, [templates])

  const addedBySection = useMemo(() => {
    const m = new Map<string, number>()
    const bump = (sec: string) => m.set(sec, (m.get(sec) ?? 0) + 1)
    const tplSection = new Map(templates.map((t) => [t.id, t.section]))
    for (const key of room.picked) {
      if (key.startsWith('n')) bump(key.slice(1))
      else {
        const sec = tplSection.get(Number(key.slice(1)))
        if (sec) bump(sec)
      }
    }
    return m
  }, [room.picked, templates])

  const q = query.trim().toLowerCase()
  const list = area === null ? [] : templates.filter((t) => t.section === area && (!q || t.name.toLowerCase().includes(q)))
  const areaLabel = area ? SECTION_LABEL[area] ?? area : ''

  const goBack = () => {
    setArea(null)
    setQuery('')
  }
  const close = () => {
    goBack()
    onClose()
  }

  const name = (t: DefectTemplate) => (
    <span className="flex min-w-0 flex-1 flex-col text-[14px] leading-snug" style={{ color: C.ink }}>
      {t.name}
      {(t.unit || t.threshold) && (
        <span className="text-[12px]" style={{ color: C.muted }}>
          {t.unit}
          {t.unit && t.threshold && ' · '}
          {t.threshold && `норма ${t.threshold}`}
        </span>
      )}
    </span>
  )

  return (
    <Drawer
      open={open}
      onClose={close}
      title={
        area === null ? (
          'Добавить дефект'
        ) : (
          <span className="flex items-center gap-2">
            <Button variant="text" className="-ml-2 h-8 px-2 sm:h-8" icon={<ArrowLeftIcon />} onClick={goBack}>
              Области
            </Button>
            <span>{areaLabel}</span>
          </span>
        )
      }
      footer={
        <>
          {area !== null && (
            <Button block onClick={() => onAddCustom(area)}>
              Свой дефект в разделе «{areaLabel}»
            </Button>
          )}
          <Button variant="primary" block onClick={close}>
            Готово
          </Button>
        </>
      }
    >
      {area === null && (
        <div className="flex flex-col">
          <span className="pb-2 text-[12px]" style={{ color: C.muted }}>Сначала область, затем дефект из её списка</span>
          {SECTIONS.map(([sec, label], i) => {
            const total = totalBySection.get(sec) ?? 0
            const count = addedBySection.get(sec) ?? 0
            return (
              <button
                key={sec}
                type="button"
                onClick={() => setArea(sec)}
                className="flex min-h-14 w-full cursor-pointer items-center gap-3 px-1 py-3 text-left"
                style={{ borderTop: i === 0 ? 'none' : `1px solid ${C.line}` }}
              >
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="text-[14px] font-semibold" style={{ color: C.ink }}>{label}</span>
                  <span className="text-[12px] tnum" style={{ color: C.muted }}>
                    {total} в справочнике{count > 0 && ` · добавлено ${count}`}
                  </span>
                </span>
                {count > 0 && (
                  <span className="grid h-5 min-w-5 place-items-center rounded-full px-1.5 text-[11px] font-bold text-white" style={{ background: C.ok }}>
                    {count}
                  </span>
                )}
                <span style={{ color: C.muted }}><ChevronRightIcon /></span>
              </button>
            )
          })}
        </div>
      )}

      {area !== null && (
        <div className="flex flex-col gap-2">
          <div className="relative">
            <TextInput placeholder="Поиск в разделе" value={query} onChange={(e) => setQuery(e.target.value)} className="pl-9" />
            <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2" style={{ color: C.faint }}><SearchIcon /></span>
          </div>
          {list.map((t) => {
            if (area === 'wall') {
              const on = room.wallsOn[t.id] ?? [false, false, false, false]
              return (
                <div key={t.id} className="flex flex-col gap-2 border-t px-1 py-3" style={{ borderColor: C.line }}>
                  {name(t)}
                  <div className="flex gap-1.5" role="group" aria-label="Стены с дефектом">
                    {WALLS.map((w) => (
                      <Button
                        key={w}
                        variant={on[w] ? 'ghost-active' : 'default'}
                        aria-pressed={on[w]}
                        className="flex-1 px-2"
                        onClick={() => onToggleWall(t.id, w)}
                      >
                        Ст. {w + 1}
                      </Button>
                    ))}
                  </div>
                </div>
              )
            }
            const added = room.picked.includes(`s${t.id}`)
            return (
              <button
                key={t.id}
                type="button"
                disabled={added}
                onClick={() => onPick(t)}
                className="flex min-h-12 w-full cursor-pointer items-center gap-3 border-t px-1 py-3 text-left disabled:cursor-default"
                style={{ borderColor: C.line, opacity: added ? 0.7 : 1 }}
              >
                {name(t)}
                {added && <span className="text-[12px] font-medium" style={{ color: C.ok }}>✓ уже добавлен</span>}
              </button>
            )
          })}
          {list.length === 0 && <span className="py-3 text-[13px]" style={{ color: C.faint }}>Ничего не найдено</span>}
        </div>
      )}
    </Drawer>
  )
}
