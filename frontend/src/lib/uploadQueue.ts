// Очередь отправки фото. Живёт вне React-дерева: переживает переходы между
// страницами, шлёт файлы по одному, повторяет при обрывах связи, ждёт сеть
// и останавливается при истёкшей сессии до повторного входа.
import { useSyncExternalStore } from 'react'
import { api, ApiError, type PhotoRef } from './api'
import { compressImage } from './compressImage'

export type UploadStatus = 'queued' | 'compressing' | 'uploading' | 'done' | 'failed' | 'auth'

export interface UploadItem {
  key: string
  actId: number
  defectId: number
  name: string
  file: File
  blob?: Blob
  status: UploadStatus
  pct: number
  error: string
  attempts: number
  retryAt: number
  photo?: PhotoRef
}

const MAX_ATTEMPTS = 5
const TIMEOUT_MS = 120_000

let items: UploadItem[] = []
let running = false
let paused = false
let seq = 0
let timer: ReturnType<typeof setTimeout> | null = null
const listeners = new Set<() => void>()

function emit() {
  items = [...items]
  listeners.forEach((l) => l())
}

function patch(key: string, changes: Partial<UploadItem>) {
  const it = items.find((i) => i.key === key)
  if (it) Object.assign(it, changes)
  emit()
}

function isRetryable(e: unknown) {
  if (!(e instanceof ApiError)) return true
  return e.status === 0 || e.status === 408 || e.status === 429 || e.status >= 500
}

function backoff(attempts: number) {
  return Math.min(2000 * 2 ** (attempts - 1), 30_000)
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
  if (running || paused) return
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
      const c = await compressImage(next.file)
      next.blob = c.blob
      next.name = c.name
    }
    await waitForNetwork()
    patch(next.key, { status: 'uploading', pct: 0, error: '' })
    const photo = await api.uploadPhoto(next.defectId, next.blob!, next.name, (pct) => patch(next.key, { pct }), TIMEOUT_MS)
    patch(next.key, { status: 'done', pct: 100, photo })
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'Ошибка загрузки'
    if (e instanceof ApiError && e.status === 401) {
      paused = true
      patch(next.key, { status: 'auth', error: msg })
    } else if (isRetryable(e) && next.attempts + 1 < MAX_ATTEMPTS) {
      const attempts = next.attempts + 1
      patch(next.key, { status: 'queued', attempts, retryAt: Date.now() + backoff(attempts), error: msg })
    } else {
      patch(next.key, { status: 'failed', error: msg })
    }
  } finally {
    running = false
  }
  void pump()
}

export const uploadQueue = {
  enqueue(actId: number, defectId: number, files: File[]) {
    for (const file of files) {
      items.push({
        key: `u${++seq}`,
        actId,
        defectId,
        name: file.name,
        file,
        status: 'queued',
        pct: 0,
        error: '',
        attempts: 0,
        retryAt: 0,
      })
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
      if (i.status === 'failed') Object.assign(i, { status: 'queued', attempts: 0, retryAt: 0, error: '' })
    })
    emit()
    void pump()
  },
  remove(key: string) {
    items = items.filter((i) => i.key !== key)
    emit()
  },
  ack(key: string) {
    items = items.filter((i) => i.key !== key)
    emit()
  },
  rebind(oldDefectId: number, newDefectId: number) {
    items.forEach((i) => {
      if (i.defectId === oldDefectId && i.status !== 'done') i.defectId = newDefectId
    })
    emit()
  },
  // После повторного входа: снимаем паузу и возвращаем остановленные файлы в очередь
  resume() {
    paused = false
    items.forEach((i) => {
      if (i.status === 'auth') Object.assign(i, { status: 'queued', retryAt: 0, error: '' })
    })
    emit()
    void pump()
  },
  pause() {
    paused = true
  },
  snapshot: () => items,
}

window.addEventListener('online', () => void pump())
window.addEventListener('beforeunload', (e) => {
  if (items.some((i) => i.status !== 'done' && i.status !== 'failed')) e.preventDefault()
})

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
