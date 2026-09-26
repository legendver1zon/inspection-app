// Очередь отправки фото. Живёт вне React-дерева и в IndexedDB: переживает
// переходы, перезагрузку и закрытие вкладки. Фото адресуется ключом
// помещение/раздел/шаблон/стена, а не id дефекта (id меняются при каждом
// сохранении формы). Шлёт по одному, повторяет при обрывах, ждёт сеть,
// при истёкшей сессии останавливается до входа.
import { useSyncExternalStore } from 'react'
import { createStore, del, entries, set } from 'idb-keyval'
import { api, ApiError, type PhotoRef } from './api'
import { compressImage } from './compressImage'

export type UploadStatus = 'queued' | 'compressing' | 'uploading' | 'done' | 'failed' | 'auth'

export interface PhotoKey {
  actId: number
  roomNumber: number
  section: string
  templateId: number | null
  wallNumber: number
}

export interface UploadItem extends PhotoKey {
  key: string
  clientId: string
  name: string
  file?: File
  blob?: Blob
  status: UploadStatus
  pct: number
  error: string
  attempts: number
  retryAt: number
  photo?: PhotoRef
  defectId?: number
}

const MAX_ATTEMPTS = 6
const TIMEOUT_MS = 120_000
const store = createStore('inspection-app', 'uploads')

let items: UploadItem[] = []
let running = false
let paused = false
let held = false
let timer: ReturnType<typeof setTimeout> | null = null
const listeners = new Set<() => void>()

function emit() {
  items = [...items]
  listeners.forEach((l) => l())
}

function persist(it: UploadItem) {
  if (it.status === 'done') {
    void del(it.key, store).catch(() => {})
    return
  }
  const { key, clientId, actId, roomNumber, section, templateId, wallNumber, name, file, blob, status, attempts, error } = it
  void set(key, { key, clientId, actId, roomNumber, section, templateId, wallNumber, name, file: blob ? undefined : file, blob, status, attempts, error }, store).catch(() => {})
}

function patch(key: string, changes: Partial<UploadItem>) {
  const it = items.find((i) => i.key === key)
  if (!it) return
  Object.assign(it, changes)
  persist(it)
  emit()
}

function newId() {
  const rnd = crypto.getRandomValues(new Uint8Array(8))
  return Date.now().toString(36) + '-' + Array.from(rnd, (b) => b.toString(16).padStart(2, '0')).join('')
}

// 404 тоже повторяем: помещение появится на сервере после автосохранения формы
function isRetryable(e: unknown) {
  if (!(e instanceof ApiError)) return true
  return e.status === 0 || e.status === 404 || e.status === 408 || e.status === 429 || e.status >= 500
}

function backoff(attempts: number) {
  return Math.min(3000 * 2 ** (attempts - 1), 60_000)
}

async function waitForNetwork() {
  if (navigator.onLine) return
  await new Promise<void>((resolve) => {
    const on = () => {
      window.removeEventListener('online', on)
      resolve()
    }
    window.addEventListener('online', on)
  })
}

function schedule(ms: number) {
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    timer = null
    void pump()
  }, ms)
}

async function pump() {
  await ready
  if (running || paused || held) return
  const now = Date.now()
  const next = items.find((i) => i.status === 'queued' && i.retryAt <= now)
  if (!next) {
    const later = items.filter((i) => i.status === 'queued').map((i) => i.retryAt)
    if (later.length) schedule(Math.max(50, Math.min(...later) - now))
    return
  }
  running = true
  try {
    if (!next.blob) {
      patch(next.key, { status: 'compressing' })
      const c = await compressImage(next.file!)
      next.blob = c.blob
      next.name = c.name
      next.file = undefined
      persist(next)
    }
    await waitForNetwork()
    patch(next.key, { status: 'uploading', pct: 0, error: '' })
    const res = await api.uploadPhotoByKey(next, next.blob!, next.name, next.clientId, (pct) => patch(next.key, { pct }), TIMEOUT_MS)
    patch(next.key, { status: 'done', pct: 100, photo: res.photo, defectId: res.defectId })
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'Ошибка загрузки'
    if (e instanceof ApiError && e.status === 401) {
      paused = true
      patch(next.key, { status: 'auth', error: msg })
    } else if (isRetryable(e) && next.attempts + 1 < MAX_ATTEMPTS) {
      const attempts = next.attempts + 1
      patch(next.key, { status: 'queued', attempts, retryAt: Date.now() + backoff(attempts), error: msg })
    } else if (isRetryable(e)) {
      // Сеть так и не появилась: оставляем в очереди, попробуем при online или вручную
      patch(next.key, { status: 'failed', error: msg })
    } else {
      patch(next.key, { status: 'failed', error: msg })
    }
  } finally {
    running = false
  }
  void pump()
}

// Восстановление очереди из IndexedDB при старте приложения
const ready: Promise<void> = (async () => {
  try {
    const rows = await entries<string, Partial<UploadItem>>(store)
    const restored: UploadItem[] = []
    for (const [, r] of rows) {
      if (!r || !r.key || (!r.blob && !r.file)) continue
      restored.push({
        key: r.key,
        clientId: r.clientId ?? newId(),
        actId: r.actId!,
        roomNumber: r.roomNumber!,
        section: r.section!,
        templateId: r.templateId ?? null,
        wallNumber: r.wallNumber ?? 0,
        name: r.name ?? 'photo.jpg',
        file: r.file,
        blob: r.blob,
        status: r.status === 'failed' ? 'failed' : 'queued',
        pct: 0,
        error: r.error ?? '',
        attempts: r.attempts ?? 0,
        retryAt: 0,
      })
    }
    items = [...restored, ...items]
    emit()
  } catch {
    /* IndexedDB недоступна — очередь живёт только в памяти */
  }
})()

export const uploadQueue = {
  ready,
  enqueue(k: PhotoKey, files: File[]) {
    for (const file of files) {
      const it: UploadItem = {
        ...k,
        key: 'u' + newId(),
        clientId: newId(),
        name: file.name,
        file,
        status: 'queued',
        pct: 0,
        error: '',
        attempts: 0,
        retryAt: 0,
      }
      items.push(it)
      persist(it)
    }
    emit()
    void pump()
  },
  retry(key: string) {
    patch(key, { status: 'queued', attempts: 0, retryAt: 0, error: '' })
    void pump()
  },
  retryAll() {
    items.forEach((i) => {
      if (i.status === 'failed') {
        Object.assign(i, { status: 'queued', attempts: 0, retryAt: 0, error: '' })
        persist(i)
      }
    })
    emit()
    void pump()
  },
  remove(key: string) {
    const it = items.find((i) => i.key === key)
    items = items.filter((i) => i.key !== key)
    if (it) void del(it.key, store).catch(() => {})
    emit()
  },
  ack(key: string) {
    items = items.filter((i) => i.key !== key)
    emit()
  },
  // Помещение удалено из формы: его фото убираем, номера ниже сдвигаем
  renumberRooms(actId: number, removedRoomNumber: number) {
    items = items.filter((i) => {
      if (i.actId !== actId || i.roomNumber !== removedRoomNumber || i.status === 'done') return true
      void del(i.key, store).catch(() => {})
      return false
    })
    items.forEach((i) => {
      if (i.actId === actId && i.roomNumber > removedRoomNumber && i.status !== 'done') {
        i.roomNumber -= 1
        persist(i)
      }
    })
    emit()
  },
  resume() {
    paused = false
    items.forEach((i) => {
      if (i.status === 'auth') {
        Object.assign(i, { status: 'queued', retryAt: 0, error: '' })
        persist(i)
      }
    })
    emit()
    void pump()
  },
  pause() {
    paused = true
  },
  // На время сохранения формы (дефекты пересоздаются) отправку не начинаем
  hold() {
    held = true
  },
  release() {
    held = false
    void pump()
  },
  kick() {
    void pump()
  },
  snapshot: () => items,
}

window.addEventListener('online', () => {
  items.forEach((i) => {
    if (i.status === 'failed' && i.attempts >= MAX_ATTEMPTS - 1) Object.assign(i, { status: 'queued', attempts: 0, retryAt: 0 })
  })
  emit()
  void pump()
})
void ready.then(() => pump())

const subscribe = (l: () => void) => {
  listeners.add(l)
  return () => listeners.delete(l)
}

export function useUploadQueue(): UploadItem[] {
  return useSyncExternalStore(subscribe, uploadQueue.snapshot, uploadQueue.snapshot)
}

export function isActive(i: UploadItem) {
  return i.status === 'queued' || i.status === 'compressing' || i.status === 'uploading'
}

export function sameKey(i: PhotoKey, k: PhotoKey) {
  return i.actId === k.actId && i.roomNumber === k.roomNumber && i.section === k.section && (i.templateId ?? null) === (k.templateId ?? null) && i.wallNumber === k.wallNumber
}
