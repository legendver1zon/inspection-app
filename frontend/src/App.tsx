import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence } from 'framer-motion'
import { api, ApiError } from './lib/api'
import Login from './pages/Login'
import Inspections from './pages/Inspections'
import ActView from './pages/ActView'
import EditAct from './pages/EditAct'

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
    <AnimatePresence mode="wait">
      <Routes location={location} key={location.pathname}>
        <Route path="/login" element={authed ? <Navigate to="/inspections" replace /> : <Login />} />
        <Route
          path="/inspections"
          element={
            checking ? (
              <PageLoader />
            ) : authed ? (
              <Inspections user={me.data!.user} />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        />
        <Route
          path="/inspections/:id"
          element={
            checking ? (
              <PageLoader />
            ) : authed ? (
              <ActView user={me.data!.user} />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        />
        <Route
          path="/inspections/:id/edit"
          element={
            checking ? (
              <PageLoader />
            ) : authed ? (
              <EditAct user={me.data!.user} />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        />
        <Route path="*" element={<Navigate to={authed ? '/inspections' : '/login'} replace />} />
      </Routes>
    </AnimatePresence>
  )
}

function PageLoader() {
  return (
    <div className="grid min-h-dvh place-items-center text-muted">
      <div className="flex items-center gap-3">
        <span className="size-2 animate-pulse rounded-full bg-accent" />
        Загрузка…
      </div>
    </div>
  )
}
