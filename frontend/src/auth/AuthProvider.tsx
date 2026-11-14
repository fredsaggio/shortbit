import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getCurrentUser, login, logout } from '../api/auth.ts'
import { ApiError, isAbortError } from '../api/client.ts'
import type { User } from '../api/types.ts'
import { AuthContext, type AuthState } from './AuthContext.ts'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })

  const loadUser = useCallback(async (signal?: AbortSignal): Promise<User | null> => {
    try {
      const user = await getCurrentUser(signal)
      setState({ status: 'authenticated', user })
      return user
    } catch (error) {
      if (isAbortError(error)) {
        throw error
      }
      // Any failure, not only 401, leaves the visitor signed out: the pages
      // that need a session send them to the login screen.
      setState({ status: 'anonymous' })
      return null
    }
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    getCurrentUser(controller.signal).then(
      (user) => setState({ status: 'authenticated', user }),
      (error: unknown) => {
        if (!isAbortError(error)) {
          setState({ status: 'anonymous' })
        }
      },
    )
    return () => controller.abort()
  }, [])

  const signIn = useCallback(
    async (email: string, password: string, rememberMe: boolean) => {
      await login(email, password, rememberMe)
      const user = await loadUser()
      if (!user) {
        throw new ApiError(0, 'Login feito, mas a sessão não foi reconhecida. Tente de novo.')
      }
    },
    [loadUser],
  )

  const signOut = useCallback(async () => {
    try {
      await logout()
    } finally {
      setState({ status: 'anonymous' })
    }
  }, [])

  const value = useMemo(
    () => ({ state, refresh: () => loadUser(), signIn, signOut }),
    [state, loadUser, signIn, signOut],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
