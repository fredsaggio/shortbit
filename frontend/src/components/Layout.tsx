import { useState } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import { useAuth } from '../auth/AuthContext.ts'
import { Logo } from './Logo.tsx'

const currentYear = new Date().getFullYear()

export function Layout() {
  const { state, signOut } = useAuth()
  const navigate = useNavigate()
  const [signingOut, setSigningOut] = useState(false)

  async function handleSignOut() {
    setSigningOut(true)
    try {
      await signOut()
    } catch {
      // The local session is cleared anyway; the cookie expires on its own.
    } finally {
      setSigningOut(false)
      navigate('/')
    }
  }

  return (
    <div className="app">
      <a className="skip-link" href="#conteudo">
        Pular para o conteúdo
      </a>
      <header className="header">
        <div className="container header__inner">
          <Link to="/" className="header__brand" aria-label="ShortBit, página inicial">
            <Logo />
          </Link>
          <nav className="header__nav" aria-label="Principal">
            {state.status === 'authenticated' ? (
              <>
                <NavLink to="/painel" className="nav-link">
                  Meus links
                </NavLink>
                <span className="header__user" title={state.user.email}>
                  {state.user.email}
                </span>
                <button type="button" className="button button--ghost" onClick={handleSignOut} disabled={signingOut}>
                  Sair
                </button>
              </>
            ) : state.status === 'anonymous' ? (
              <>
                <NavLink to="/entrar" className="nav-link">
                  Entrar
                </NavLink>
                <Link to="/cadastro" className="button button--primary">
                  Criar conta
                </Link>
              </>
            ) : null}
          </nav>
        </div>
      </header>

      <main id="conteudo" className="main">
        <Outlet />
      </main>

      <footer className="footer">
        <div className="container footer__inner">
          <Logo />
          <p className="footer__text">Links curtos, públicos ou protegidos por senha, com cliques contados.</p>
          <p className="footer__text footer__text--muted">Projeto de estudo · {currentYear}</p>
        </div>
      </footer>
    </div>
  )
}
