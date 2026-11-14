import { apiRequest } from './client.ts'
import type { User } from './types.ts'

// A full page navigation, not fetch: the API answers with a redirect to Google.
export const googleLoginPath = '/auth/google'

export function getCurrentUser(signal?: AbortSignal): Promise<User> {
  return apiRequest<User>('/me', { signal })
}

export function login(email: string, password: string, rememberMe: boolean): Promise<void> {
  return apiRequest<void>('/sessions', {
    method: 'POST',
    body: { email, password, remember_me: rememberMe },
  })
}

export function logout(): Promise<void> {
  return apiRequest<void>('/sessions/current', { method: 'DELETE' })
}

export function startRegistration(email: string, password: string): Promise<void> {
  return apiRequest<void>('/registrations/password', {
    method: 'POST',
    body: { email, password },
  })
}

export function confirmRegistration(code: string): Promise<User> {
  return apiRequest<User>('/registrations/password/confirm', {
    method: 'POST',
    body: { code },
  })
}

export function resendRegistrationCode(): Promise<void> {
  return apiRequest<void>('/registrations/password/resend', { method: 'POST' })
}

export function startPasswordReset(email: string): Promise<void> {
  return apiRequest<void>('/password-resets', {
    method: 'POST',
    body: { email },
  })
}

export function confirmPasswordReset(code: string, password: string, passwordConfirmation: string): Promise<void> {
  return apiRequest<void>('/password-resets/confirm', {
    method: 'POST',
    body: { code, password, password_confirmation: passwordConfirmation },
  })
}
