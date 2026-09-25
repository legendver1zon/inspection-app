import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence } from 'framer-motion'
import { useEffect, type ReactNode } from 'react'
import { api, ApiError, AUTH_EXPIRED_EVENT, type User } from './lib/api'
import { uploadQueue } from './lib/uploadQueue'
import Login from './pages/Login'
import Register from './pages/Register'
import ForgotPassword from './pages/ForgotPassword'
import ResetPassword from './pages/ResetPassword'
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
  const queryClient = useQueryClient()

  // Сессия истекла посреди работы: очередь фото ставится на паузу,
  // а пользователь уходит на вход и после него возвращается на ту же страницу
  useEffect(() => {
    const onExpired = () => {
      uploadQueue.pause()
      queryClient.setQueryData(['me'], null)
    }
    window.addEventListener(AUTH_EXPIRED_EVENT, onExpired)
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired)
  }, [queryClient])

  const user = me.data?.user
  const checking = me.isLoading

  // Куда вести после входа: на страницу, с которой выкинуло, иначе в ленту.
  // Тот же адрес использует и сам Login, иначе уходящее при анимации дерево
  // маршрутов успевало бы перебить переход своим Navigate.
  const from = (location.state as { from?: string } | null)?.from
  const afterLogin = <Navigate to={from && from !== '/login' ? from : '/inspections'} replace />

  // Охрана: пока проверяем сессию — лоадер; без сессии — на логин
  const guard = (render: (u: User) => ReactNode, adminOnly = false) => {
    if (checking) return <PageLoader />
    if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />
    if (adminOnly && user.role !== 'admin') return <Navigate to="/inspections" replace />
    return render(user)
  }

  return (
    <AnimatePresence mode="wait">
      <Routes location={location} key={location.pathname}>
        <Route path="/login" element={user ? afterLogin : <Login />} />
        <Route path="/register" element={user ? afterLogin : <Register />} />
        <Route path="/forgot-password" element={user ? afterLogin : <ForgotPassword />} />
        <Route path="/reset-password" element={user ? afterLogin : <ResetPassword />} />
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
