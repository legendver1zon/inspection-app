import { useEffect, useRef, useState, type PointerEvent } from 'react'
import { C } from '../../lib/palette'
import { Button, Drawer } from './ui'

// Поле для рукописной подписи: рисуем пальцем на canvas, отдаём PNG,
// обрезанный по чернилам, как data-URL.
export default function SignaturePad({ open, title, onClose, onDone }: {
  open: boolean
  title: string
  onClose: () => void
  onDone: (dataUrl: string) => void
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  const last = useRef<{ x: number; y: number } | null>(null)
  const [empty, setEmpty] = useState(true)

  useEffect(() => {
    if (!open) return
    setEmpty(true)
    let raf = 0
    const setup = () => {
      const c = canvasRef.current
      if (!c || !c.clientWidth) {
        raf = requestAnimationFrame(setup)
        return
      }
      const dpr = window.devicePixelRatio || 1
      c.width = Math.round(c.clientWidth * dpr)
      c.height = Math.round(c.clientHeight * dpr)
      const ctx = c.getContext('2d')!
      ctx.scale(dpr, dpr)
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.lineWidth = 2.5
      ctx.strokeStyle = '#1d1a16'
    }
    setup()
    return () => cancelAnimationFrame(raf)
  }, [open])

  const pos = (e: PointerEvent<HTMLCanvasElement>) => {
    const r = e.currentTarget.getBoundingClientRect()
    return { x: e.clientX - r.left, y: e.clientY - r.top }
  }
  const down = (e: PointerEvent<HTMLCanvasElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId)
    const p = pos(e)
    const ctx = e.currentTarget.getContext('2d')!
    ctx.beginPath()
    ctx.moveTo(p.x, p.y)
    ctx.lineTo(p.x + 0.1, p.y + 0.1)
    ctx.stroke()
    drawing.current = true
    last.current = p
    setEmpty(false)
  }
  const move = (e: PointerEvent<HTMLCanvasElement>) => {
    if (!drawing.current || !last.current) return
    const p = pos(e)
    const ctx = e.currentTarget.getContext('2d')!
    ctx.beginPath()
    ctx.moveTo(last.current.x, last.current.y)
    ctx.lineTo(p.x, p.y)
    ctx.stroke()
    last.current = p
  }
  const up = () => {
    drawing.current = false
    last.current = null
  }

  function clear() {
    const c = canvasRef.current
    if (!c) return
    const ctx = c.getContext('2d')!
    ctx.save()
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, c.width, c.height)
    ctx.restore()
    setEmpty(true)
  }

  function exportPng(): string | null {
    const c = canvasRef.current
    if (!c) return null
    const { width, height } = c
    const data = c.getContext('2d')!.getImageData(0, 0, width, height).data
    let minX = width, minY = height, maxX = -1, maxY = -1
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        if (data[(y * width + x) * 4 + 3] === 0) continue
        if (x < minX) minX = x
        if (x > maxX) maxX = x
        if (y < minY) minY = y
        if (y > maxY) maxY = y
      }
    }
    if (maxX < 0) return null
    const pad = 16
    const sx = Math.max(0, minX - pad), sy = Math.max(0, minY - pad)
    const sw = Math.min(width, maxX + pad) - sx, sh = Math.min(height, maxY + pad) - sy
    const scale = Math.min(1, 900 / sw)
    const out = document.createElement('canvas')
    out.width = Math.max(1, Math.round(sw * scale))
    out.height = Math.max(1, Math.round(sh * scale))
    out.getContext('2d')!.drawImage(c, sx, sy, sw, sh, 0, 0, out.width, out.height)
    return out.toDataURL('image/png')
  }

  const done = () => {
    const d = exportPng()
    if (d) onDone(d)
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      action={<Button variant="primary" className="h-9 px-3 sm:h-9" disabled={empty} onClick={done}>Готово</Button>}
      footer={
        <div className="flex gap-2">
          <Button block onClick={clear}>Очистить</Button>
          <Button variant="primary" block disabled={empty} onClick={done}>Готово</Button>
        </div>
      }
    >
      <p className="text-[13px]" style={{ color: C.muted }}>Распишитесь пальцем или стилусом в поле ниже и нажмите «Готово».</p>
      <div className="mt-3 overflow-hidden rounded-xl border" style={{ borderColor: C.line, background: '#fff' }}>
        <canvas
          ref={canvasRef}
          className="block h-[min(240px,45dvh)] w-full touch-none"
          aria-label="Поле для подписи"
          onPointerDown={down}
          onPointerMove={move}
          onPointerUp={up}
          onPointerCancel={up}
        />
      </div>
    </Drawer>
  )
}
