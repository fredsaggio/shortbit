import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { useAuth } from '../auth/AuthContext.ts'
import { PageLoader } from './PageLoader.tsx'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { state } = useAuth()
  const location = useLocation()

  if (state.status === 'loading') {
    return <PageLoader />
  }
  if (state.status === 'anonymous') {
    return <Navigate to="/entrar" replace state={{ from: location.pathname }} />
  }
  return children
}

export function GuestOnly({ children }: { children: ReactNode }) {
  const { state } = useAuth()

  if (state.status === 'loading') {
    return <PageLoader />
  }
  if (state.status === 'authenticated') {
    return <Navigate to="/painel" replace />
  }
  return children
}
