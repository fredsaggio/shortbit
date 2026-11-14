import { ChevronDown, LayoutDashboard, LogOut, Menu, UserRound } from 'lucide-react'
import { useState } from 'react'
import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import type { Account } from '../api/types.ts'
import { useAuth } from '../auth/AuthContext.ts'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { buttonSizes, buttonTones } from './buttonStyles.ts'
import { Logo } from './Logo.tsx'

const currentYear = new Date().getFullYear()

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'inline-flex h-10 items-center gap-2 rounded-md px-3 text-[15px] font-semibold transition-colors duration-150',
    isActive ? 'bg-white/15 text-white' : 'text-navy-300 hover:bg-white/10 hover:text-white',
  )

function initials(account: Account): string {
  const source = account.full_name?.trim() || account.email
  const parts = source.split(/[\s@.]+/).filter(Boolean)
  return ((parts[0]?.[0] ?? '') + (account.full_name ? (parts[1]?.[0] ?? '') : '')).toUpperCase()
}

function UserMenu({ account, onSignOut }: { account: Account; onSignOut: () => void }) {
  const navigate = useNavigate()
  const firstName = account.full_name?.split(' ')[0]

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="group inline-flex h-10 items-center gap-2 rounded-full py-1 pr-3 pl-1 text-white transition-colors duration-150 hover:bg-white/10 data-[state=open]:bg-white/15"
        aria-label={`Menu da conta${firstName ? ` de ${firstName}` : ''}`}
      >
        <Avatar className="size-8">
          <AvatarFallback className="bg-brand text-sm font-bold text-white">{initials(account)}</AvatarFallback>
        </Avatar>
        <span className="hidden max-w-32 truncate text-[15px] font-semibold sm:inline">{firstName ?? 'Minha conta'}</span>
        <ChevronDown className="size-4 transition-transform duration-200 group-data-[state=open]:rotate-180" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="w-64 rounded-xl p-1.5 shadow-raised">
        <DropdownMenuLabel className="px-2.5 py-2">
          <p className="truncate text-sm font-semibold text-navy">{account.full_name ?? 'Conta sem nome'}</p>
          <p className="truncate text-xs font-normal text-muted-foreground">{account.email}</p>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem className="h-10 cursor-pointer gap-2.5 rounded-md px-2.5 font-medium" onSelect={() => navigate('/painel')}>
          <LayoutDashboard className="size-4" aria-hidden="true" /> Meus links
        </DropdownMenuItem>
        <DropdownMenuItem className="h-10 cursor-pointer gap-2.5 rounded-md px-2.5 font-medium" onSelect={() => navigate('/conta')}>
          <UserRound className="size-4" aria-hidden="true" /> Minha conta
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="h-10 cursor-pointer gap-2.5 rounded-md px-2.5 font-medium text-destructive focus:bg-destructive/5 focus:text-destructive"
          onSelect={onSignOut}
        >
          <LogOut className="size-4" aria-hidden="true" /> Sair
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function MobileMenu({ signedIn, onSignOut }: { signedIn: boolean; onSignOut: () => void }) {
  const [open, setOpen] = useState(false)
  const close = () => setOpen(false)
  const itemClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      'flex h-12 items-center rounded-md px-3 text-base font-semibold transition-colors duration-150',
      isActive ? 'bg-navy-100 text-navy' : 'text-navy hover:bg-surface',
    )

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        className="inline-flex size-10 items-center justify-center rounded-md text-white transition-colors hover:bg-white/10 md:hidden"
        aria-label="Abrir menu"
      >
        <Menu className="size-6" aria-hidden="true" />
      </SheetTrigger>
      <SheetContent side="right" className="w-[300px] bg-card p-0">
        <SheetHeader className="border-b p-5">
          <SheetTitle>
            <Logo />
          </SheetTitle>
          <SheetDescription className="sr-only">Navegação principal</SheetDescription>
        </SheetHeader>
        <nav className="grid gap-1 p-3" aria-label="Principal">
          <NavLink to="/" end className={itemClass} onClick={close}>
            Início
          </NavLink>
          {signedIn ? (
            <>
              <NavLink to="/painel" className={itemClass} onClick={close}>
                Meus links
              </NavLink>
              <NavLink to="/conta" className={itemClass} onClick={close}>
                Minha conta
              </NavLink>
              <button
                type="button"
                className="flex h-12 items-center rounded-md px-3 text-left text-base font-semibold text-destructive transition-colors hover:bg-destructive/5"
                onClick={() => {
                  close()
                  onSignOut()
                }}
              >
                Sair
              </button>
            </>
          ) : (
            <>
              <NavLink to="/entrar" className={itemClass} onClick={close}>
                Entrar
              </NavLink>
              <Link to="/cadastro" onClick={close} className={cn('mt-2 inline-flex items-center justify-center rounded-md font-semibold', buttonSizes.md, buttonTones.primary)}>
                Criar conta grátis
              </Link>
            </>
          )}
        </nav>
      </SheetContent>
    </Sheet>
  )
}

export function Layout() {
  const { state, signOut } = useAuth()
  const navigate = useNavigate()

  async function handleSignOut() {
    try {
      await signOut()
    } catch {
      // The local session is cleared anyway; the cookie expires on its own.
    }
    navigate('/')
  }

  return (
    <div className="flex min-h-screen flex-col">
      <a
        href="#conteudo"
        className="sr-only z-50 rounded-md bg-white px-4 py-2 font-semibold text-navy focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        Pular para o conteúdo
      </a>

      <header className="sticky top-0 z-40 border-b border-white/10 bg-navy">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6">
          <Link to="/" aria-label="shortbit, página inicial" className="rounded-md">
            <Logo tone="light" />
          </Link>

          <nav className="hidden items-center gap-1 md:flex" aria-label="Principal">
            {state.status === 'authenticated' && (
              <NavLink to="/painel" className={navLinkClass}>
                Meus links
              </NavLink>
            )}
          </nav>

          <div className="flex items-center gap-2">
            {state.status === 'authenticated' ? (
              <UserMenu account={state.user} onSignOut={handleSignOut} />
            ) : state.status === 'anonymous' ? (
              <div className="hidden items-center gap-2 md:flex">
                <NavLink to="/entrar" className={navLinkClass}>
                  Entrar
                </NavLink>
                <Link to="/cadastro" className={cn('inline-flex items-center rounded-md font-semibold transition-colors', buttonSizes.sm, 'h-10 px-4', buttonTones.onDark)}>
                  Criar conta grátis
                </Link>
              </div>
            ) : null}
            {state.status !== 'loading' && <MobileMenu signedIn={state.status === 'authenticated'} onSignOut={handleSignOut} />}
          </div>
        </div>
      </header>

      <main id="conteudo" className="flex-1">
        <Outlet />
      </main>

      <footer className="bg-navy text-navy-300">
        <div className="mx-auto grid max-w-7xl gap-8 px-4 py-12 sm:px-6 md:grid-cols-[1.5fr_1fr_1fr]">
          <div>
            <Logo tone="light" />
            <p className="mt-4 max-w-sm text-sm leading-relaxed">
              Encurte links públicos ou protegidos por senha e veja quantas pessoas clicaram.
            </p>
          </div>
          <div>
            <h2 className="text-sm font-semibold tracking-wide text-white uppercase">Produto</h2>
            <ul className="mt-4 grid gap-2.5 text-sm">
              {(state.status === 'authenticated'
                ? [
                    { to: '/painel', label: 'Meus links' },
                    { to: '/conta', label: 'Minha conta' },
                  ]
                : [
                    { to: '/cadastro', label: 'Criar conta' },
                    { to: '/entrar', label: 'Entrar' },
                  ]
              ).map((item) => (
                <li key={item.to}>
                  <Link to={item.to} className="rounded-sm transition-colors hover:text-white">
                    {item.label}
                  </Link>
                </li>
              ))}
              <li>
                <a href="/docs" className="rounded-sm transition-colors hover:text-white">
                  Documentação da API
                </a>
              </li>
            </ul>
          </div>
          <div>
            <h2 className="text-sm font-semibold tracking-wide text-white uppercase">Privacidade</h2>
            <p className="mt-4 text-sm leading-relaxed">
              Usamos apenas cookies essenciais, para manter você conectado e proteger o login. Nada de rastreamento de
              terceiros.
            </p>
          </div>
        </div>
        <div className="border-t border-white/10">
          <p className="mx-auto max-w-7xl px-4 py-5 text-xs sm:px-6">© {currentYear} shortbit · Projeto de estudo</p>
        </div>
      </footer>
    </div>
  )
}
