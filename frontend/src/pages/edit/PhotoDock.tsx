import { useEffect, useMemo, useRef } from 'react'
import { api } from '../../lib/api'
import { C } from '../../lib/palette'
import { isActive, uploadQueue, useUploadQueue, type PhotoKey } from '../../lib/uploadQueue'
import PhotoThumb from '../../components/PhotoThumb'
import { Button, CameraIcon, CloseIcon } from './ui'
import type { DefectBind } from './form'

export interface BindRef {
  key: string
  bind: DefectBind
}

const previews = new Map<string, string>()
function previewUrl(key: string, blob?: Blob) {
  if (!blob) return undefined
  let u = previews.get(key)
  if (!u) {
    u = URL.createObjectURL(blob)
    previews.set(key, u)
  }
  return u
}

// Фото дефекта: миниатюры, очередь отправки (с превью), кнопка «Добавить фото».
// Загрузка адресуется ключом помещение/раздел/шаблон/стена, поэтому фото
// можно снимать сразу после выбора дефекта и без сети — уйдут позже.
export default function PhotoDock({ k, bindKey, binds, canUpload, hint, onBind }: {
  k: PhotoKey
  bindKey: string
  binds: BindRef[]
  canUpload: boolean
  hint?: string
  onBind: (key: string, b: DefectBind) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const all = useUploadQueue()
  const items = useMemo(
    () => all.filter((i) => i.actId === k.actId && i.roomNumber === k.roomNumber && i.section === k.section && (i.templateId ?? null) === (k.templateId ?? null)),
    [all, k.actId, k.roomNumber, k.section, k.templateId],
  )

  // Готовые фото из очереди переносим в привязку дефекта (создавая её, если
  // дефект появился на сервере только что)
  useEffect(() => {
    for (const it of items) {
      if (it.status !== 'done' || !it.photo) continue
      const targetKey = k.section === 'wall' && it.wallNumber > 0 ? `${bindKey.split('_')[0]}_${it.wallNumber - 1}` : bindKey
      const ref = binds.find((b) => b.key === targetKey)
      if (ref) {
        if (!ref.bind.photos.some((p) => p.id === it.photo!.id)) onBind(targetKey, { ...ref.bind, photos: [...ref.bind.photos, it.photo] })
      } else {
        onBind(targetKey, { defectId: it.defectId ?? 0, photos: [it.photo] })
      }
      uploadQueue.ack(it.key)
      const u = previews.get(it.key)
      if (u) {
        URL.revokeObjectURL(u)
        previews.delete(it.key)
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items])

  const photos = binds.flatMap((b) => b.bind.photos.map((p) => ({ p, key: b.key, bind: b.bind }))).sort((a, b) => a.p.id - b.p.id)
  const total = photos.length
  const pending = items.filter((u) => u.status !== 'done')
  const failed = pending.find((u) => u.status === 'failed' || u.status === 'auth')
  const sending = pending.filter(isActive).length

  async function remove(key: string, bind: DefectBind, photoId: number) {
    if (!window.confirm('Удалить фото?')) return
    try {
      await api.deletePhoto(photoId)
      onBind(key, { ...bind, photos: bind.photos.filter((p) => p.id !== photoId) })
    } catch {
      /* останется в списке */
    }
  }

  function handleFiles(files: FileList | null) {
    if (!files) return
    uploadQueue.enqueue(k, Array.from(files))
    if (inputRef.current) inputRef.current.value = ''
  }

  return (
    <div className="flex flex-col gap-2.5">
      {(total > 0 || pending.length > 0) && (
        <div className="flex flex-wrap gap-2">
          {photos.map(({ p, key, bind }) => (
            <span key={p.id} className="relative block size-[84px] overflow-hidden rounded-lg border sm:size-20" style={{ borderColor: C.line }}>
              <a href={`/photos/${p.id}/download`} target="_blank" rel="noreferrer">
                <PhotoThumb id={p.id} className="size-full object-cover" />
              </a>
              {p.status !== 'done' && (
                <span className="absolute bottom-1 left-1 rounded px-1 text-[10px] font-bold text-white" style={{ background: p.status === 'failed' ? C.err : C.warn }}>
                  {p.status === 'failed' ? 'сбой' : '↑'}
                </span>
              )}
              <button
                type="button"
                onClick={() => remove(key, bind, p.id)}
                aria-label="Удалить фото"
                className="absolute top-1 right-1 grid size-7 cursor-pointer place-items-center rounded-md text-white"
                style={{ background: C.err }}
              >
                <CloseIcon />
              </button>
            </span>
          ))}
          {pending.map((u) => {
            const bad = u.status === 'failed' || u.status === 'auth'
            const src = previewUrl(u.key, u.blob ?? u.file)
            return (
              <span
                key={u.key}
                title={u.error || u.name}
                className="relative block size-[84px] overflow-hidden rounded-lg border sm:size-20"
                style={{ borderColor: bad ? C.err : C.line, background: C.track }}
              >
                {src && <img src={src} alt="" className="size-full object-cover" style={{ opacity: 0.55 }} />}
                <span
                  className="absolute inset-x-0 bottom-0 px-1 py-0.5 text-center text-[11px] font-bold"
                  style={{ background: 'rgba(255,255,255,.85)', color: bad ? C.err : C.muted }}
                >
                  {u.status === 'uploading' ? `${u.pct}%` : u.status === 'compressing' ? '…' : bad ? 'не ушло' : navigator.onLine ? 'в очереди' : 'ждёт сеть'}
                </span>
                {bad && (
                  <button
                    type="button"
                    onClick={() => uploadQueue.remove(u.key)}
                    aria-label="Убрать из очереди"
                    className="absolute top-1 right-1 grid size-7 cursor-pointer place-items-center rounded-md text-white"
                    style={{ background: C.err }}
                  >
                    <CloseIcon />
                  </button>
                )}
              </span>
            )
          })}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Button icon={<CameraIcon />} disabled={!canUpload} onClick={() => inputRef.current?.click()}>
          {total > 0 || pending.length > 0 ? 'Ещё фото' : 'Добавить фото'}
        </Button>
        {(total > 0 || sending > 0) && (
          <span className="text-[12px]" style={{ color: C.muted }}>
            {total > 0 && `${total} фото`}
            {sending > 0 && `${total > 0 ? ' · ' : ''}отправляется ${sending}`}
          </span>
        )}
        {!canUpload && hint && <span className="text-[12px]" style={{ color: C.faint }}>{hint}</span>}
        <input ref={inputRef} type="file" accept="image/jpeg,image/png,image/webp" multiple hidden onChange={(e) => handleFiles(e.target.files)} />
      </div>

      {failed && (
        <div className="flex items-center gap-2 rounded-lg px-2.5 py-1.5" style={{ background: C.errBg }}>
          <span className="min-w-0 flex-1 truncate text-[12px]" style={{ color: C.err }}>
            {failed.status === 'auth' ? 'Войдите заново, отправка продолжится' : failed.error}
          </span>
          {failed.status === 'failed' && (
            <Button variant="danger-text" className="h-8 px-2 text-[12px] sm:h-8" onClick={() => uploadQueue.retry(failed.key)}>
              Повторить
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
