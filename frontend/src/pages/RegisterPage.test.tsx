import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

async function fillDetails(actions: ReturnType<typeof renderApp>['user'], confirmation = 'senha12345') {
  await actions.type(await screen.findByLabelText('Email'), user.email)
  await actions.type(screen.getByLabelText('Senha'), 'senha12345')
  await actions.type(screen.getByLabelText('Confirme a senha'), confirmation)
  await actions.click(screen.getByRole('button', { name: 'Continuar' }))
}

describe('RegisterPage', () => {
  it('confirms the code and signs the new account in', async () => {
    let signedIn = false
    const api = fakeApi({
      'GET /me': () => (signedIn ? { status: 200, body: user } : { status: 401 }),
      'POST /registrations/password': { status: 202 },
      'POST /registrations/password/confirm': { status: 201, body: user },
      'POST /sessions': () => {
        signedIn = true
        return { status: 204 }
      },
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
    })
    const { user: actions } = renderApp('/cadastro')

    await fillDetails(actions)
    expect(await screen.findByText(`Enviamos um código de 6 dígitos para ${user.email}.`)).toBeInTheDocument()

    await actions.type(screen.getByLabelText('Código de confirmação'), '12a3456')
    await actions.click(screen.getByRole('button', { name: 'Confirmar e entrar' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/painel'))
    expect(api.calls('POST', '/registrations/password')[0].body).toEqual({ email: user.email, password: 'senha12345' })
    expect(api.calls('POST', '/registrations/password/confirm')[0].body).toEqual({ code: '123456' })
    expect(api.calls('POST', '/sessions')[0].body).toEqual({ email: user.email, password: 'senha12345', remember_me: false })
  })

  it('does not call the API when the passwords differ', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/cadastro')

    await fillDetails(actions, 'outra-senha1')

    const confirmation = screen.getByLabelText('Confirme a senha')
    expect(await screen.findByText('As senhas não coincidem.')).toBeInTheDocument()
    expect(confirmation).toHaveAttribute('aria-invalid', 'true')
    expect(confirmation).toHaveFocus()
    expect(api.calls('POST', '/registrations/password')).toHaveLength(0)
  })

  it('shows a taken email', async () => {
    fakeApi({ ...signedOut, 'POST /registrations/password': { status: 409, body: 'email já está em uso' } })
    const { user: actions } = renderApp('/cadastro')

    await fillDetails(actions)

    expect(await screen.findByRole('alert')).toHaveTextContent('Email já está em uso')
  })
})
