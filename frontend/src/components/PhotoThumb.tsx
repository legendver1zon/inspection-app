import { useState, type CSSProperties } from 'react'

// Миниатюра фото: лёгкий /thumb, при сбое — оригинал
export default function PhotoThumb({ id, className, style }: { id: number; className?: string; style?: CSSProperties }) {
  const [fallback, setFallback] = useState(false)
  return (
    <img
      src={fallback ? `/photos/${id}/download` : `/photos/${id}/thumb`}
      alt=""
      loading="lazy"
      decoding="async"
      className={className}
      style={style}
      onError={() => setFallback(true)}
    />
  )
}
