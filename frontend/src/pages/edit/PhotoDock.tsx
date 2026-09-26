import { useEffect, useRef } from 'react'
import { api } from '../../lib/api'
import { C } from '../../lib/palette'
import { isActive, uploadQueue, useUploadQueue } from '../../lib/uploadQueue'
import PhotoThumb from '../../components/PhotoThumb'
import { Button, CameraIcon, CloseIcon } from './ui'
import type { DefectBind } from './form'

export interface BindRef {
  key: string
  bind: DefectBind
}

// Фото дефекта: миниатюры, очередь отправки, кнопка «Добавить фото».
// Для стенового дефекта binds — записи всех отмеченных стен, загрузка идёт
// в первую из них.
export default function PhotoDock({ actId, binds, canUpload, hint, onBind }: {
  actId: number
  binds: BindRef[]
  canUpload: boolean
  hint?: string
  onBind: (key: string, b: DefectBind) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const ids = binds.map((b) => b.bind.defectId)
  const items = useUploadQueue().filter((i) => ids.includes(i.defectId))
  const target = binds[0]
  const prevTarget = useRef(target?.bind.defectId)

  // Автосейв пересоздаёт дефекты с новыми id — переводим очередь на новый id
  useEffect(() => {
    const cur = target?.bind.defectId
    if (prevTarget.current && cur && prevTarget.current !== cur) uploadQueue.rebind(prevTarget.current, cur)
    prevTarget.current = cur
  }, [target?.bind.defectId])

  // Готовые фото из очереди переносим в привязку дефекта
  useEffect(() => {
    for (const it of items) {
      if (it.status !== 'done' || !it.photo) continue
      const ref = binds.find((b) => b.bind.defectId === it.defectId) ?? target
      if (ref && !ref.bind.photos.some((p) => p.id === it.photo!.id)) {
        onBind(ref.key, { ...ref.bind, photos: [...ref.bind.photos, it.photo] })
      }
      uploadQueue.ack(it.key)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items])

  const photos = binds.flatMap((b) => b.bind.photos.map((p) => ({ p, key: b.key, bind: b.bind }))).sort((a, b) => a.p.id - b.p.id)
  const total = photos.length
  const pending = items.filter((u) => u.status !== 'done')
  const failed = pending.find((u) => u.status === 'failed' || u.status === 'auth')

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
    if (!files || !target) return
    uploadQueue.enqueue(actId, target.bind.defectId, Array.from(files))
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
            return (
              <span
                key={u.key}
                title={u.error || u.name}
                className="relative grid size-[84px] place-items-center rounded-lg border text-[12px] font-semibold sm:size-20"
                style={{ borderColor: bad ? C.err : C.line, background: C.track, color: bad ? C.err : C.muted }}
              >
                {u.status === 'uploading' ? `${u.pct}%` : u.status === 'compressing' ? '…' : bad ? '!' : u.attempts > 0 ? `↻${u.attempts}` : '⏳'}
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
        <Button icon={<CameraIcon />} disabled={!canUpload || !target} onClick={() => inputRef.current?.click()}>
          {total > 0 ? 'Ещё фото' : 'Добавить фото'}
        </Button>
        {total > 0 && (
          <span className="text-[12px]" style={{ color: C.muted }}>
            {total} фото{pending.some(isActive) ? ` · отправляется ${pending.filter(isActive).length}` : ''}
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
