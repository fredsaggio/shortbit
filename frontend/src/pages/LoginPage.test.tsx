import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

describe('LoginPage', () => {
  it('signs in and opens the dashboard', async () => {
    let signedIn = false
    const api = fakeApi({
      'GET /me': () => (signedIn ? { status: 200, body: user } : { status: 401 }),
      'POST /sessions': () => {
        signedIn = true
        return { status: 204 }
      },
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
    })
    const { user: actions } = renderApp('/entrar')

    await actions.type(await screen.findByLabelText('Email'), user.email)
    await actions.type(screen.getByLabelText('Senha'), 'senha12345')
    await actions.click(screen.getByLabelText('Manter conectado por 30 dias'))
    await actions.click(screen.getByRole('button', { name: 'Entrar' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/painel'))
    expect(api.calls('POST', '/sessions')[0].body).toEqual({ email: user.email, password: 'senha12345', remember_me: true })
    expect(await screen.findByRole('button', { name: 'Menu da conta de Ana' })).toBeInTheDocument()
  })

  it('shows the API message when the password is wrong', async () => {
    fakeApi({ ...signedOut, 'POST /sessions': { status: 401, body: 'email ou senha incorretos' } })
    const { user: actions } = renderApp('/entrar')

    await actions.type(await screen.findByLabelText('Email'), user.email)
    await actions.type(screen.getByLabelText('Senha'), 'errada123')
    await actions.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Email ou senha incorretos')
    expect(screen.getByTestId('current-path')).toHaveTextContent('/entrar')
  })

  it('shows its own messages for empty fields instead of calling the API', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/entrar')

    await actions.click(await screen.findByRole('button', { name: 'Entrar' }))

    expect(screen.getByText('Informe seu email.')).toBeInTheDocument()
    expect(screen.getByText('Informe sua senha.')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveFocus()
    expect(api.calls('POST', '/sessions')).toHaveLength(0)

    await actions.type(screen.getByLabelText('Email'), 'ana')
    expect(screen.queryByText('Informe seu email.')).not.toBeInTheDocument()
  })

  it('rejects a malformed email before calling the API', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/entrar')

    await actions.type(await screen.findByLabelText('Email'), 'ana@')
    await actions.type(screen.getByLabelText('Senha'), 'senha12345')
    await actions.click(screen.getByRole('button', { name: 'Entrar' }))

    expect(screen.getByText('Digite um email válido, como nome@exemplo.com.')).toBeInTheDocument()
    expect(api.calls('POST', '/sessions')).toHaveLength(0)
  })

  it('explains a failed Google login', async () => {
    fakeApi(signedOut)
    renderApp('/entrar?erro=google')

    expect(await screen.findByRole('alert')).toHaveTextContent('Não foi possível entrar com o Google')
  })

  it('starts the Google login on the API', async () => {
    fakeApi(signedOut)
    renderApp('/entrar')

    expect(await screen.findByRole('link', { name: 'Continuar com Google' })).toHaveAttribute('href', '/auth/google')
  })
})
