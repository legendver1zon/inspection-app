import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'framer-motion'
import { api, ApiError, type User } from './lib/api'
import Login from './pages/Login'
import TopBar from './components/TopBar'
import { VariantProvider, VariantSwitcher, useVariant } from './concepts/VariantContext'
import V1Inspections from './concepts/V1Inspections'
import V2Inspections from './concepts/V2Inspections'
import V3Inspections from './concepts/V3Inspections'

function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    retry: (count, err) => !(err instanceof ApiError && err.status === 401) && count < 1,
  })
}

export default function App() {
  const location = useLocation()
  const me = useMe()

  const authed = !!me.data?.user
  const checking = me.isLoading

  return (
    <VariantProvider>
      <AnimatePresence mode="wait">
        <Routes location={location} key={location.pathname}>
          <Route path="/login" element={authed ? <Navigate to="/inspections" replace /> : <Login />} />
          <Route
            path="/inspections"
            element={
              checking ? (
                <PageLoader />
              ) : authed ? (
                <InspectionsVariant user={me.data!.user} />
              ) : (
                <Navigate to="/login" replace />
              )
            }
          />
          <Route path="*" element={<Navigate to={authed ? '/inspections' : '/login'} replace />} />
        </Routes>
      </AnimatePresence>
    </VariantProvider>
  )
}

// Пока идёт выбор дизайн-направления, экран списка существует в трёх
// вариантах — переключатель внизу. После решения останется один.
function InspectionsVariant({ user }: { user: User }) {
  const { variant } = useVariant()
  return (
    <>
      <AnimatePresence mode="wait">
        <motion.div
          key={variant}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.2 }}
        >
          {variant === 'v1' && <V1Inspections user={user} />}
          {variant === 'v2' && (
            <>
              <TopBar user={user} />
              <V2Inspections user={user} />
            </>
          )}
          {variant === 'v3' && <V3Inspections user={user} />}
        </motion.div>
      </AnimatePresence>
      <VariantSwitcher />
    </>
  )
}

function PageLoader() {
  return (
    <div className="grid min-h-dvh place-items-center text-muted">
      <div className="flex items-center gap-3">
        <span className="size-2 animate-pulse rounded-full bg-accent-bright" />
        Загрузка…
      </div>
    </div>
  )
}
