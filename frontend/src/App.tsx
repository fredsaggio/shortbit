import { Route, Routes } from 'react-router'
import { AuthProvider } from './auth/AuthProvider.tsx'
import { Layout } from './components/Layout.tsx'
import { GuestOnly, RequireAuth } from './components/RouteGuards.tsx'
import { DashboardPage } from './pages/DashboardPage.tsx'
import { ForgotPasswordPage } from './pages/ForgotPasswordPage.tsx'
import { HomePage } from './pages/HomePage.tsx'
import { LinkDetailPage } from './pages/LinkDetailPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { NotFoundPage } from './pages/NotFoundPage.tsx'
import { RegisterPage } from './pages/RegisterPage.tsx'

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route
            path="entrar"
            element={
              <GuestOnly>
                <LoginPage />
              </GuestOnly>
            }
          />
          <Route
            path="cadastro"
            element={
              <GuestOnly>
                <RegisterPage />
              </GuestOnly>
            }
          />
          <Route path="recuperar-senha" element={<ForgotPasswordPage />} />
          <Route
            path="painel"
            element={
              <RequireAuth>
                <DashboardPage />
              </RequireAuth>
            }
          />
          <Route
            path="painel/links/:code"
            element={
              <RequireAuth>
                <LinkDetailPage />
              </RequireAuth>
            }
          />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </AuthProvider>
  )
}
