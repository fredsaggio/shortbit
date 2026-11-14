import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

async function fillDetails(actions: ReturnType<typeof renderApp>['user'], confirmation = 'senha12345') {
  await actions.type(await screen.findByLabelText('Nome completo'), user.full_name)
  await actions.type(screen.getByLabelText('Email'), user.email)
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
    expect(api.calls('POST', '/registrations/password')[0].body).toEqual({ email: user.email, password: 'senha12345', full_name: 'Ana Souza' })
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

  it('requires the full name', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/cadastro')

    await actions.type(await screen.findByLabelText('Email'), user.email)
    await actions.click(screen.getByRole('button', { name: 'Continuar' }))

    expect(screen.getByText('Informe seu nome completo.')).toBeInTheDocument()
    expect(screen.getByLabelText('Nome completo')).toHaveFocus()
    expect(api.calls('POST', '/registrations/password')).toHaveLength(0)
  })

  it('shows a taken email', async () => {
    fakeApi({ ...signedOut, 'POST /registrations/password': { status: 409, body: 'email já está em uso' } })
    const { user: actions } = renderApp('/cadastro')

    await fillDetails(actions)

    expect(await screen.findByRole('alert')).toHaveTextContent('Email já está em uso')
  })

  it('lights up each password requirement as it is met', async () => {
    fakeApi(signedOut)
    const { user: actions } = renderApp('/cadastro')

    const checklist = await screen.findByRole('list', { name: 'Requisitos da senha' })
    const rule = (name: string) => within(checklist).getByText(name).closest('li')

    expect(rule('Pelo menos 8 caracteres')).toHaveAttribute('data-state', 'pending')
    expect(rule('As duas senhas são iguais')).toHaveAttribute('data-state', 'pending')

    await actions.type(screen.getByLabelText('Senha'), 'senha12345')
    expect(rule('Pelo menos 8 caracteres')).toHaveAttribute('data-state', 'met')
    expect(rule('15 ou mais caracteres')).toHaveAttribute('data-state', 'pending')

    await actions.type(screen.getByLabelText('Confirme a senha'), 'senha12345')
    expect(rule('As duas senhas são iguais')).toHaveAttribute('data-state', 'met')
    expect(screen.getByLabelText('Senha')).toHaveAccessibleDescription(/Pelo menos 8 caracteres/)
  })

  it('does not block sign-up on the recommended length', async () => {
    const api = fakeApi({ ...signedOut, 'POST /registrations/password': { status: 202 } })
    const { user: actions } = renderApp('/cadastro')

    await actions.type(await screen.findByLabelText('Nome completo'), 'Ana Souza')
    await actions.type(screen.getByLabelText('Email'), 'ana@example.com')
    await actions.type(screen.getByLabelText('Senha'), 'senha123')
    await actions.type(screen.getByLabelText('Confirme a senha'), 'senha123')
    await actions.click(screen.getByRole('button', { name: 'Continuar' }))

    await waitFor(() => expect(api.calls('POST', '/registrations/password')).toHaveLength(1))
  })
})
