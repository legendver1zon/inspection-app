import { motion } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import { api, type User } from '../lib/api'
import { C } from '../lib/palette'
import Header from '../components/Header'

export default function Dashboard({ user }: { user: User }) {
  const { data, isLoading } = useQuery({ queryKey: ['dashboard'], queryFn: api.dashboard })

  const tiles: { n?: number; label: string; tone?: 'accent' | 'ok' | 'warn' | 'err' }[] = [
    { n: data?.draft, label: 'в работе', tone: 'accent' },
    { n: data?.completed, label: 'завершено', tone: 'ok' },
    { n: data?.total, label: 'всего актов' },
    { n: data?.today, label: 'создано сегодня' },
    { n: data?.week, label: 'за 7 дней' },
    { n: data?.photo_pending, label: 'фото в очереди', tone: data && data.photo_pending > 0 ? 'warn' : undefined },
    { n: data?.photo_failed, label: 'сбоев выгрузки', tone: data && data.photo_failed > 0 ? 'err' : undefined },
  ]

  const toneColor = (t?: string) =>
    t === 'accent' ? C.accent : t === 'ok' ? C.ok : t === 'warn' ? C.warn : t === 'err' ? C.err : C.ink

  return (
    <div className="min-h-dvh" style={{ background: C.bg, color: C.ink }}>
      <Header user={user} />
      <main className="mx-auto max-w-5xl px-5 pt-8 pb-24">
        <h1 className="mb-1 text-[24px] font-extrabold tracking-tight">Статистика</h1>
        <p className="mb-7 text-sm" style={{ color: C.muted }}>
          {user.role === 'admin' ? 'По всем инспекторам' : 'По вашим осмотрам'}
        </p>

        {isLoading ? (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <div key={i} className="h-28 animate-pulse rounded-2xl motion-reduce:animate-none" style={{ background: C.track }} />
            ))}
          </div>
        ) : (
          <motion.div
            variants={{ show: { transition: { staggerChildren: 0.05 } } }}
            initial="hidden"
            animate="show"
            className="grid grid-cols-2 gap-4 sm:grid-cols-3"
          >
            {tiles.map((t) => (
              <motion.div
                key={t.label}
                variants={{ hidden: { opacity: 0, y: 14 }, show: { opacity: 1, y: 0 } }}
                className="rounded-2xl border px-5 py-5" style={{ background: C.surface, borderColor: C.line }}
              >
                <div className="text-[38px] leading-none font-black tracking-tight tnum" style={{ color: toneColor(t.tone) }}>
                  {t.n ?? '…'}
                </div>
                <div className="mt-2 text-[11.5px] font-bold tracking-wider uppercase" style={{ color: C.faint }}>
                  {t.label}
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}
      </main>
    </div>
  )
}
