import { useEffect, useState, type CSSProperties } from 'react'

// Миниатюра фото: лёгкий /thumb, при сбое — оригинал; после появления сети
// картинка запрашивается заново (без сети браузер её больше не перезапросит)
export default function PhotoThumb({ id, className, style }: { id: number; className?: string; style?: CSSProperties }) {
  const [fallback, setFallback] = useState(false)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    const on = () => {
      setFallback(false)
      setTick((t) => t + 1)
    }
    window.addEventListener('online', on)
    return () => window.removeEventListener('online', on)
  }, [])

  return (
    <img
      key={tick}
      src={fallback ? `/photos/${id}/download` : `/photos/${id}/thumb`}
      alt=""
      loading="lazy"
      decoding="async"
      className={className}
      style={style}
      onError={() => { if (navigator.onLine) setFallback(true) }}
    />
  )
}
