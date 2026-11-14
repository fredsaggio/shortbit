import { createContext, useContext } from 'react'
import type { User } from '../api/types.ts'

export type AuthState =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'authenticated'; user: User }

export type AuthContextValue = {
  state: AuthState
  refresh: () => Promise<User | null>
  signIn: (email: string, password: string, rememberMe: boolean) => Promise<void>
  signOut: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext)
  if (!value) {
    throw new Error('useAuth precisa estar dentro de AuthProvider')
  }
  return value
}
