import { useEffect, useRef, useState } from 'react'
import Cropper from 'cropperjs'
import 'cropperjs/dist/cropper.css'
import { api } from '../../lib/api'
import { C } from '../../lib/palette'
import { Button, Card, PlusIcon } from './ui'

export default function PlanCard({ actId, planUrl, onUploaded }: { actId: number; planUrl: string; onUploaded: () => void }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const imgRef = useRef<HTMLImageElement>(null)
  const cropperRef = useRef<Cropper | null>(null)
  const [cropSrc, setCropSrc] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  function openFile(files: FileList | null) {
    const f = files?.[0]
    if (!f) return
    setCropSrc(URL.createObjectURL(f))
    if (inputRef.current) inputRef.current.value = ''
  }

  useEffect(() => {
    if (!cropSrc || !imgRef.current) return
    const cropper = new Cropper(imgRef.current, { viewMode: 1, autoCropArea: 1, background: false })
    cropperRef.current = cropper
    return () => {
      cropper.destroy()
      URL.revokeObjectURL(cropSrc)
    }
  }, [cropSrc])

  async function confirmCrop() {
    const canvas = cropperRef.current?.getCroppedCanvas({ maxWidth: 2000, maxHeight: 2000 })
    if (!canvas) return
    setBusy(true)
    canvas.toBlob(
      async (blob) => {
        if (blob) {
          await api.uploadPlan(actId, blob)
          onUploaded()
        }
        setBusy(false)
        setCropSrc(null)
      },
      'image/jpeg',
      0.9,
    )
  }

  return (
    <Card
      title="План квартиры"
      extra={
        <>
          <Button icon={planUrl ? undefined : <PlusIcon />} onClick={() => inputRef.current?.click()}>
            {planUrl ? 'Заменить план' : 'Загрузить план'}
          </Button>
          <input ref={inputRef} type="file" accept="image/jpeg,image/png,image/webp" hidden onChange={(e) => openFile(e.target.files)} />
        </>
      }
    >
      {planUrl && !cropSrc && (
        <img src={planUrl} alt="План квартиры" className="max-h-64 rounded-lg border object-contain" style={{ borderColor: C.line }} />
      )}
      {!planUrl && !cropSrc && (
        <p className="text-[13px]" style={{ color: C.faint }}>План появится в PDF-акте — загрузите фото или скан.</p>
      )}
      {cropSrc && (
        <div className="flex flex-col gap-3">
          <div className="max-h-96 overflow-hidden rounded-lg border" style={{ borderColor: C.line }}>
            <img ref={imgRef} src={cropSrc} alt="" className="block max-w-full" />
          </div>
          <div className="flex gap-2">
            <Button variant="primary" disabled={busy} onClick={confirmCrop}>
              {busy ? 'Загружаем…' : 'Обрезать и сохранить'}
            </Button>
            <Button onClick={() => setCropSrc(null)}>Отмена</Button>
          </div>
        </div>
      )}
    </Card>
  )
}
