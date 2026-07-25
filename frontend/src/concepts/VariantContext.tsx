import { createContext, useContext, useState, type ReactNode } from 'react'
import { motion } from 'framer-motion'

// Переключатель дизайн-концептов. Живёт только на время выбора направления —
// после решения лишние варианты удаляются вместе с этим файлом.

export type Variant = 'v1' | 'v2' | 'v3'

const VariantContext = createContext<{ variant: Variant; setVariant: (v: Variant) => void }>({
  variant: 'v2',
  setVariant: () => {},
})

export function VariantProvider({ children }: { children: ReactNode }) {
  const [variant, setVariantState] = useState<Variant>(() => {
    try {
      const saved = localStorage.getItem('ui-variant')
      if (saved === 'v1' || saved === 'v2' || saved === 'v3') return saved
    } catch { /* приватный режим */ }
    return 'v2'
  })
  const setVariant = (v: Variant) => {
    setVariantState(v)
    try {
      localStorage.setItem('ui-variant', v)
    } catch { /* приватный режим */ }
  }
  return <VariantContext.Provider value={{ variant, setVariant }}>{children}</VariantContext.Provider>
}

export function useVariant() {
  return useContext(VariantContext)
}

const LABELS: Record<Variant, string> = { v1: '1 · Линия', v2: '2 · Поле', v3: '3 · Студия' }

export function VariantSwitcher() {
  const { variant, setVariant } = useVariant()
  return (
    <div className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2">
      <div className="flex gap-1 rounded-full border border-black/10 bg-white/85 p-1 shadow-[0_8px_30px_rgba(0,0,0,.18)] backdrop-blur-lg dark:border-white/15 dark:bg-black/60">
        {(Object.keys(LABELS) as Variant[]).map((v) => (
          <button
            key={v}
            onClick={() => setVariant(v)}
            className={`relative cursor-pointer rounded-full px-4 py-2 text-[13px] font-bold whitespace-nowrap transition-colors ${
              variant === v ? 'text-white' : 'text-neutral-600 hover:text-neutral-900 dark:text-neutral-300 dark:hover:text-white'
            }`}
          >
            {variant === v && (
              <motion.span
                layoutId="variant-pill"
                className="absolute inset-0 rounded-full bg-neutral-900 dark:bg-white/25"
                transition={{ type: 'spring', stiffness: 500, damping: 35 }}
              />
            )}
            <span className="relative">{LABELS[v]}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
