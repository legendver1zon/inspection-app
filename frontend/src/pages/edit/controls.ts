import type { CSSProperties } from 'react'
import { C } from '../../lib/palette'

export const controlCls = 'h-10 w-full min-w-0 rounded-lg border px-3 text-[14px] outline-none transition-shadow focus:ring-2 sm:h-9 sm:text-[13px]'

export function controlStyle(invalid = false): CSSProperties {
  return {
    background: C.surface,
    borderColor: invalid ? C.err : C.line,
    color: C.ink,
    ['--tw-ring-color' as string]: C.accentSoft,
  }
}
