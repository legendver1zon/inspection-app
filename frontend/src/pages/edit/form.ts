import type { DefectTemplate, EditRoomData, PhotoRef } from '../../lib/api'

export const SECTIONS: [string, string][] = [
  ['window', 'Окна и откосы'],
  ['ceiling', 'Потолок'],
  ['wall', 'Стены'],
  ['floor', 'Пол'],
  ['door', 'Двери'],
  ['plumbing', 'Сантехника'],
]
export const SECTION_LABEL: Record<string, string> = Object.fromEntries(SECTIONS)

export const WINDOW_TYPES: [string, string][] = [
  ['', '—'],
  ['pvc', 'ПВХ'],
  ['al', 'Алюминий'],
  ['wood', 'Дерево'],
]
export const WALL_TYPES: [string, string][] = [
  ['paint', 'окраска'],
  ['tile', 'плитка'],
  ['gkl', 'ГКЛ'],
]
export const WALLS = [0, 1, 2, 3] as const

// Привязка сохранённого дефекта (id + фото) к полю формы:
// ключи "s{tmplId}" | "w{tmplId}_{wall0-3}" | "n{section}"
export interface DefectBind {
  defectId: number
  photos: PhotoRef[]
}

export type Measures = {
  length: string; width: string; height: string; dh: string; dw: string
  w1h: string; w1w: string; w2h: string; w2w: string; w3h: string; w3w: string
  w4h: string; w4w: string; w5h: string; w5w: string
}

export interface RoomForm {
  key: number
  name: string
  m: Measures
  windowType: string
  wallTypes: string[]
  simple: Record<number, string>
  walls: Record<number, [string, string, string, string]>
  wallsOn: Record<number, [boolean, boolean, boolean, boolean]>
  notes: Record<string, string>
  // Что показано карточками: "s{tpl}" | "w{tpl}" | "n{section}". Дефект без
  // значения на сервере не создаётся, поэтому выбор живёт в форме.
  picked: string[]
  binds: Record<string, DefectBind>
}

let roomKeySeq = 1

export function numStr(v: number): string {
  return v ? String(v) : ''
}

export function num(s: string): number {
  const n = Number(String(s).replace(',', '.'))
  return Number.isFinite(n) ? n : 0
}

export function plural(n: number, one: string, few: string, many: string) {
  const m10 = n % 10
  const m100 = n % 100
  if (m10 === 1 && m100 !== 11) return one
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few
  return many
}

export function emptyRoom(): RoomForm {
  return {
    key: roomKeySeq++,
    name: '',
    m: { length: '', width: '', height: '', dh: '', dw: '',
         w1h: '', w1w: '', w2h: '', w2w: '', w3h: '', w3w: '', w4h: '', w4w: '', w5h: '', w5w: '' },
    windowType: '',
    wallTypes: [],
    simple: {},
    walls: {},
    wallsOn: {},
    notes: {},
    picked: [],
    binds: {},
  }
}

// bindsFrom строит привязки «поле формы → сохранённый дефект (id + фото)».
// Используется и при загрузке, и после автосейва (дефекты пересоздаются).
export function bindsFrom(r: EditRoomData): Record<string, DefectBind> {
  const binds: Record<string, DefectBind> = {}
  for (const d of r.defects) {
    let bindKey: string
    if (d.template_id == null) bindKey = `n${d.section}`
    else if (d.section === 'wall' && d.wall_number >= 1 && d.wall_number <= 4)
      bindKey = `w${d.template_id}_${d.wall_number - 1}`
    else bindKey = `s${d.template_id}`
    binds[bindKey] = { defectId: d.id, photos: d.photos ?? [] }
  }
  return binds
}

export function roomFromData(r: EditRoomData): RoomForm {
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
  const picked = new Set<string>()
  for (const d of r.defects) {
    if (d.template_id == null) {
      const txt = d.notes || d.value
      if (txt) room.notes[d.section] = room.notes[d.section] ? `${room.notes[d.section]}; ${txt}` : txt
      picked.add(`n${d.section}`)
    } else if (d.section === 'wall' && d.wall_number >= 1 && d.wall_number <= 4) {
      const arr = room.walls[d.template_id] ?? ['', '', '', '']
      arr[d.wall_number - 1] = d.value
      room.walls[d.template_id] = arr
      const on = room.wallsOn[d.template_id] ?? [false, false, false, false]
      on[d.wall_number - 1] = true
      room.wallsOn[d.template_id] = on
      picked.add(`w${d.template_id}`)
    } else {
      room.simple[d.template_id] = d.value
      picked.add(`s${d.template_id}`)
    }
  }
  room.picked = [...picked]
  room.binds = bindsFrom(r)
  return room
}

export function buildParams(header: Record<string, string>, rooms: RoomForm[]): URLSearchParams {
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
  return p
}

export function measured(r: RoomForm) {
  return num(r.m.length) > 0 && num(r.m.width) > 0 && num(r.m.height) > 0
}

export function windowsCount(r: RoomForm) {
  let n = 0
  for (const w of [1, 2, 3, 4, 5]) {
    if (r.m[`w${w}h` as keyof Measures] || r.m[`w${w}w` as keyof Measures]) n++
  }
  return n
}

// Сколько дефектов реально уйдёт в акт (с непустым значением)
export function defectCount(r: RoomForm) {
  let n = Object.values(r.simple).filter((v) => v.trim()).length
  for (const vals of Object.values(r.walls)) if (vals.some((v) => v.trim())) n++
  n += Object.values(r.notes).filter((v) => v.trim()).length
  return n
}

export function photoCount(r: RoomForm) {
  return Object.values(r.binds).reduce((s, b) => s + b.photos.length, 0)
}

export function tplMap(templates: DefectTemplate[]) {
  return new Map(templates.map((t) => [t.id, t]))
}
