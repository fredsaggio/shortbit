import { createContext, useContext } from 'react'
import type { Account } from '../api/types.ts'

export type AuthState =
  | { status: 'loading' }
  | { status: 'anonymous' }
  | { status: 'authenticated'; user: Account }

export type AuthContextValue = {
  state: AuthState
  refresh: () => Promise<Account | null>
  replaceAccount: (account: Account) => void
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
