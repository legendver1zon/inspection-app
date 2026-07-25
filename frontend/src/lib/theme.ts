import { useCallback, useSyncExternalStore } from 'react'

// Тема хранится в localStorage, класс .dark на <html> ставится
// и до рендера (index.html), и при переключении здесь.

let listeners: Array<() => void> = []

function currentTheme(): 'light' | 'dark' {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

function subscribe(cb: () => void) {
  listeners.push(cb)
  return () => {
    listeners = listeners.filter((l) => l !== cb)
  }
}

export function useTheme() {
  const theme = useSyncExternalStore(subscribe, currentTheme)
  const toggle = useCallback(() => {
    const next = currentTheme() === 'dark' ? 'light' : 'dark'
    document.documentElement.classList.toggle('dark', next === 'dark')
    try {
      localStorage.setItem('theme', next)
    } catch {
      /* приватный режим — просто не сохраняем */
    }
    listeners.forEach((l) => l())
  }, [])
  return { theme, toggle }
}
