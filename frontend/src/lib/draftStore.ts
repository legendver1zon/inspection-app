// Черновик редактора акта в IndexedDB: правки не пропадают без сети,
// при перезагрузке и после закрытия вкладки. Стирается после успешного
// сохранения на сервере.
import { createStore, del, get, set } from 'idb-keyval'
import type { DefectTemplate } from './api'

// Своя база: idb-keyval создаёт хранилище только при первом открытии базы
const store = createStore('inspection-app-drafts', 'drafts')

export interface DraftRoom {
  name: string
  m: Record<string, string>
  windowType: string
  wallTypes: string[]
  simple: Record<number, string>
  walls: Record<number, [string, string, string, string]>
  wallsOn: Record<number, [boolean, boolean, boolean, boolean]>
  notes: Record<string, string>
  picked: string[]
}

export interface Draft {
  actId: number
  actNumber: string
  header: Record<string, string>
  rooms: DraftRoom[]
  templates: DefectTemplate[]
  updatedAt: number
}

export async function loadDraft(actId: number): Promise<Draft | undefined> {
  try {
    return await get<Draft>(`act:${actId}`, store)
  } catch {
    return undefined
  }
}

export async function saveDraft(d: Draft) {
  try {
    await set(`act:${d.actId}`, d, store)
  } catch {
    /* нет IndexedDB — черновик только в памяти */
  }
}

export async function clearDraft(actId: number) {
  try {
    await del(`act:${actId}`, store)
  } catch {
    /* ignore */
  }
}
