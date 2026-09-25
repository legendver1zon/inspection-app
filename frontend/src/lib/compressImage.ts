// Сжатие фото на телефоне перед отправкой: снимок 2–4 МБ превращается
// в 300–500 КБ, что в разы ускоряет загрузку на слабой связи.

const MAX_SIDE = 1920
const QUALITY = 0.82
const SKIP_BELOW = 600 * 1024

export interface Compressed {
  blob: Blob
  name: string
}

async function decode(file: File): Promise<ImageBitmap | HTMLImageElement> {
  if ('createImageBitmap' in window) {
    try {
      return await createImageBitmap(file, { imageOrientation: 'from-image' })
    } catch {
      /* старые браузеры без опции — ниже запасной путь */
    }
  }
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => {
      URL.revokeObjectURL(url)
      resolve(img)
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('decode'))
    }
    img.src = url
  })
}

function jpegName(name: string) {
  return name.replace(/\.[^.]+$/, '') + '.jpg'
}

export async function compressImage(file: File): Promise<Compressed> {
  if (!/^image\/(jpeg|png|webp)$/.test(file.type)) return { blob: file, name: file.name }
  try {
    const src = await decode(file)
    const w = 'naturalWidth' in src ? src.naturalWidth : src.width
    const h = 'naturalHeight' in src ? src.naturalHeight : src.height
    const scale = Math.min(1, MAX_SIDE / Math.max(w, h))
    if (scale === 1 && file.type === 'image/jpeg' && file.size <= SKIP_BELOW) {
      return { blob: file, name: file.name }
    }
    const tw = Math.max(1, Math.round(w * scale))
    const th = Math.max(1, Math.round(h * scale))
    const canvas = document.createElement('canvas')
    canvas.width = tw
    canvas.height = th
    const ctx = canvas.getContext('2d')
    if (!ctx) return { blob: file, name: file.name }
    ctx.drawImage(src, 0, 0, tw, th)
    if ('close' in src) src.close()
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', QUALITY))
    if (!blob || blob.size === 0) return { blob: file, name: file.name }
    return { blob, name: jpegName(file.name) }
  } catch {
    return { blob: file, name: file.name }
  }
}
