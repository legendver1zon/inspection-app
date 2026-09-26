import { useEffect, useRef, useState, type PointerEvent } from 'react'
import { C } from '../../lib/palette'
import { Button, Drawer } from './ui'

const PAD_H = 220
const INK = '#1d1a16'

// Границы чернил в пикселях битмапа (по альфа-каналу)
function inkBox(c: HTMLCanvasElement) {
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
  return maxX < 0 ? null : { minX, minY, maxX, maxY }
}

// Поле для рукописной подписи: рисуем пальцем на canvas, отдаём PNG,
// обрезанный по чернилам, как data-URL. Размер холста в пикселях фиксирован
// (иначе на телефоне он менялся бы вместе с высотой экрана и рисунок плыл),
// при повороте экрана нарисованное сохраняется.
export default function SignaturePad({ open, title, onClose, onDone }: {
  open: boolean
  title: string
  onClose: () => void
  onDone: (dataUrl: string) => void
}) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  // Штрих ведёт один указатель: второй палец, ладонь или большой палец на
  // краю экрана не должны ни начинать линию, ни обрывать её
  const activeId = useRef<number | null>(null)
  const activeType = useRef('')
  const last = useRef<{ x: number; y: number } | null>(null)
  const inked = useRef(false)
  const cssSize = useRef({ w: 0, h: 0 })
  const [empty, setEmpty] = useState(true)

  useEffect(() => {
    if (!open) return
    setEmpty(true)
    inked.current = false
    activeId.current = null
    last.current = null
    const c = canvasRef.current
    if (!c) return
    c.getContext('2d')!.clearRect(0, 0, c.width, c.height)
    let raf = 0
    const fit = () => {
      const w = c.clientWidth, h = c.clientHeight
      if (!w || !h) {
        raf = requestAnimationFrame(fit)
        return
      }
      const dpr = window.devicePixelRatio || 1
      const bw = Math.round(w * dpr), bh = Math.round(h * dpr)
      if (c.width === bw && c.height === bh) return
      let snapshot: HTMLCanvasElement | null = null
      const prev = cssSize.current
      let ink: { right: number; bottom: number } | null = null
      if (inked.current && c.width && c.height) {
        const box = inkBox(c)
        if (box) {
          const prevDpr = c.width / (prev.w || 1)
          ink = { right: (box.maxX + 1) / prevDpr, bottom: (box.maxY + 1) / prevDpr }
          snapshot = document.createElement('canvas')
          snapshot.width = c.width
          snapshot.height = c.height
          snapshot.getContext('2d')!.drawImage(c, 0, 0)
        }
      }
      c.width = bw
      c.height = bh
      cssSize.current = { w, h }
      const ctx = c.getContext('2d')!
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
      ctx.lineCap = 'round'
      ctx.lineJoin = 'round'
      ctx.lineWidth = 2.5
      ctx.strokeStyle = INK
      // Снимок возвращаем 1:1; уменьшаем равномерно только если чернила не
      // влезают, поэтому повторные повороты подпись не ужимают. Начатый штрих завершаем
      if (snapshot && ink && prev.w && prev.h) {
        const k = Math.min(1, w / ink.right, h / ink.bottom)
        ctx.drawImage(snapshot, 0, 0, prev.w * k, prev.h * k)
      }
      activeId.current = null
      last.current = null
    }
    fit()
    const ro = new ResizeObserver(() => fit())
    ro.observe(c)
    // Страховка от прокрутки и pull-to-refresh под пальцем там, где touch-action игнорируют
    const stop = (e: TouchEvent) => e.preventDefault()
    c.addEventListener('touchstart', stop, { passive: false })
    c.addEventListener('touchmove', stop, { passive: false })
    return () => {
      cancelAnimationFrame(raf)
      ro.disconnect()
      c.removeEventListener('touchstart', stop)
      c.removeEventListener('touchmove', stop)
    }
  }, [open])

  const ctxOf = (c: HTMLCanvasElement) => c.getContext('2d')!

  const down = (e: PointerEvent<HTMLCanvasElement>) => {
    if (e.button !== 0) return
    const c = e.currentTarget
    if (activeId.current !== null) {
      // Штрих уже идёт: чужие касания игнорируем, кроме двух случаев —
      // захват потерян без pointerup (окно потеряло фокус) или пришло перо,
      // а штрих начала ладонь
      const stale = !c.hasPointerCapture(activeId.current)
      const penOverPalm = e.pointerType === 'pen' && activeType.current === 'touch'
      if (!stale && !penOverPalm) return
    }
    try {
      c.setPointerCapture(e.pointerId)
    } catch {
      return
    }
    activeId.current = e.pointerId
    activeType.current = e.pointerType
    const r = c.getBoundingClientRect()
    const p = { x: e.clientX - r.left, y: e.clientY - r.top }
    const ctx = ctxOf(c)
    ctx.beginPath()
    ctx.moveTo(p.x, p.y)
    ctx.lineTo(p.x + 0.1, p.y + 0.1)
    ctx.stroke()
    last.current = p
    inked.current = true
    setEmpty(false)
  }
  const move = (e: PointerEvent<HTMLCanvasElement>) => {
    if (e.pointerId !== activeId.current || !last.current) return
    // Кнопку отпустили вне окна и pointerup не пришёл — штрих закончен
    if (e.pointerType !== 'touch' && e.buttons === 0) {
      activeId.current = null
      last.current = null
      return
    }
    const c = e.currentTarget
    const r = c.getBoundingClientRect()
    // Промежуточные точки между кадрами — линия без изломов
    const native = e.nativeEvent
    const events: globalThis.PointerEvent[] = native.getCoalescedEvents?.() ?? []
    const points = (events.length ? events : [native]).map((ev) => ({ x: ev.clientX - r.left, y: ev.clientY - r.top }))
    const ctx = ctxOf(c)
    ctx.beginPath()
    ctx.moveTo(last.current.x, last.current.y)
    for (const p of points) ctx.lineTo(p.x, p.y)
    ctx.stroke()
    last.current = points[points.length - 1]
  }
  const up = (e: PointerEvent<HTMLCanvasElement>) => {
    if (e.pointerId !== activeId.current) return
    activeId.current = null
    last.current = null
  }

  function clear() {
    const c = canvasRef.current
    if (!c) return
    const ctx = ctxOf(c)
    ctx.save()
    ctx.setTransform(1, 0, 0, 1, 0, 0)
    ctx.clearRect(0, 0, c.width, c.height)
    ctx.restore()
    inked.current = false
    activeId.current = null
    last.current = null
    setEmpty(true)
  }

  function exportPng(): string | null {
    const c = canvasRef.current
    if (!c) return null
    const { width, height } = c
    const box = inkBox(c)
    if (!box) return null
    const { minX, minY, maxX, maxY } = box
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
      <p className="text-[13px]" style={{ color: C.muted }}>Распишитесь на линии пальцем или стилусом и нажмите «Готово».</p>
      <div className="relative mt-3 overflow-hidden rounded-xl border" style={{ borderColor: C.line, background: '#fff', height: PAD_H }}>
        {/* Линия-опора: лежит под холстом и в PNG не попадает */}
        <div aria-hidden className="pointer-events-none absolute inset-x-5 border-b border-dashed" style={{ bottom: 64, borderColor: '#b5ada2' }}>
          <span className="absolute -top-3.5 -left-1 text-[16px] leading-none" style={{ color: '#b5ada2' }}>×</span>
        </div>
        <canvas
          ref={canvasRef}
          className="relative block h-full w-full touch-none select-none"
          aria-label="Поле для подписи"
          onPointerDown={down}
          onPointerMove={move}
          onPointerUp={up}
          onPointerCancel={up}
          onLostPointerCapture={up}
          onContextMenu={(e) => e.preventDefault()}
        />
      </div>
    </Drawer>
  )
}
