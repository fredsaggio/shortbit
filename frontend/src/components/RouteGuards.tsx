import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { useAuth } from '../auth/AuthContext.ts'
import { returnPath } from '../navigation.ts'
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

// Signing in on a guest page re-renders this guard first, so it honors the
// page the visitor was trying to reach.
export function GuestOnly({ children }: { children: ReactNode }) {
  const { state } = useAuth()
  const location = useLocation()

  if (state.status === 'loading') {
    return <PageLoader />
  }
  if (state.status === 'authenticated') {
    return <Navigate to={returnPath(location.state)} replace />
  }
  return children
}
