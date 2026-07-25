import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence } from 'framer-motion'
import type { ReactNode } from 'react'
import { api, ApiError, type User } from './lib/api'
import Login from './pages/Login'
import Inspections from './pages/Inspections'
import ActView from './pages/ActView'
import EditAct from './pages/EditAct'
import Dashboard from './pages/Dashboard'
import Profile from './pages/Profile'
import AdminUsers from './pages/AdminUsers'

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

  const user = me.data?.user
  const checking = me.isLoading

  // Охрана: пока проверяем сессию — лоадер; без сессии — на логин
  const guard = (render: (u: User) => ReactNode, adminOnly = false) => {
    if (checking) return <PageLoader />
    if (!user) return <Navigate to="/login" replace />
    if (adminOnly && user.role !== 'admin') return <Navigate to="/inspections" replace />
    return render(user)
  }

  return (
    <AnimatePresence mode="wait">
      <Routes location={location} key={location.pathname}>
        <Route path="/login" element={user ? <Navigate to="/inspections" replace /> : <Login />} />
        <Route path="/inspections" element={guard((u) => <Inspections user={u} />)} />
        <Route path="/inspections/:id" element={guard((u) => <ActView user={u} />)} />
        <Route path="/inspections/:id/edit" element={guard((u) => <EditAct user={u} />)} />
        <Route path="/dashboard" element={guard((u) => <Dashboard user={u} />)} />
        <Route path="/profile" element={guard((u) => <Profile user={u} />)} />
        <Route path="/admin/users" element={guard((u) => <AdminUsers user={u} />, true)} />
        <Route path="*" element={<Navigate to={user ? '/inspections' : '/login'} replace />} />
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
