import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

describe('AccountPage', () => {
  it('requires a session', async () => {
    fakeApi(signedOut)
    renderApp('/conta')

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/entrar'))
  })

  it('shows the account information', async () => {
    fakeApi({ 'GET /me': { status: 200, body: { ...user, has_google: true } } })
    renderApp('/conta')

    const info = await screen.findByRole('region', { name: 'Informações' })
    expect(within(info).getByText('ana@example.com')).toBeInTheDocument()
    expect(within(info).getAllByText('Ana Souza')).not.toHaveLength(0)
    expect(within(info).getByText('Email e senha')).toBeInTheDocument()
    expect(within(info).getByText('Google')).toBeInTheDocument()
    expect(within(info).getByText('14 de novembro de 2026')).toBeInTheDocument()
  })

  it('updates the full name and the greeting', async () => {
    const api = fakeApi({
      'GET /me': { status: 200, body: user },
      'PATCH /me': ({ body }) => ({ status: 200, body: { ...user, full_name: (body as { full_name: string }).full_name } }),
    })
    const { user: actions } = renderApp('/conta')

    const field = await screen.findByLabelText('Nome completo')
    await actions.clear(field)
    await actions.type(field, 'Beatriz Lima')
    await actions.click(screen.getByRole('button', { name: 'Salvar nome' }))

    expect(await screen.findByText('Nome atualizado.')).toBeInTheDocument()
    expect(api.calls('PATCH', '/me')[0].body).toEqual({ full_name: 'Beatriz Lima' })
    expect(screen.getByRole('button', { name: 'Menu da conta de Beatriz' })).toBeInTheDocument()
  })

  it('validates the name before calling the API', async () => {
    const api = fakeApi({ 'GET /me': { status: 200, body: user } })
    const { user: actions } = renderApp('/conta')

    await actions.clear(await screen.findByLabelText('Nome completo'))
    await actions.click(screen.getByRole('button', { name: 'Salvar nome' }))

    expect(screen.getByText('Informe seu nome completo.')).toBeInTheDocument()
    expect(api.calls('PATCH', '/me')).toHaveLength(0)
  })

  it('changes the password', async () => {
    const api = fakeApi({ 'GET /me': { status: 200, body: user }, 'POST /me/password': { status: 204 } })
    const { user: actions } = renderApp('/conta')

    await actions.click(await screen.findByRole('tab', { name: 'Segurança' }))
    await actions.type(screen.getByLabelText('Senha atual'), 'senha12345')
    await actions.type(screen.getByLabelText('Nova senha'), 'nova-senha-1')
    await actions.type(screen.getByLabelText('Confirme a nova senha'), 'nova-senha-1')
    await actions.click(screen.getByRole('button', { name: 'Trocar senha' }))

    expect(await screen.findByText('Senha alterada. Os outros aparelhos foram desconectados.')).toBeInTheDocument()
    expect(api.calls('POST', '/me/password')[0].body).toEqual({
      current_password: 'senha12345',
      new_password: 'nova-senha-1',
      new_password_confirmation: 'nova-senha-1',
    })
    expect(screen.getByLabelText('Senha atual')).toHaveValue('')
  })

  it('shows the API message for a wrong current password', async () => {
    fakeApi({ 'GET /me': { status: 200, body: user }, 'POST /me/password': { status: 400, body: 'senha atual incorreta' } })
    const { user: actions } = renderApp('/conta')

    await actions.click(await screen.findByRole('tab', { name: 'Segurança' }))
    await actions.type(screen.getByLabelText('Senha atual'), 'errada-123')
    await actions.type(screen.getByLabelText('Nova senha'), 'nova-senha-1')
    await actions.type(screen.getByLabelText('Confirme a nova senha'), 'nova-senha-1')
    await actions.click(screen.getByRole('button', { name: 'Trocar senha' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Senha atual incorreta')
  })

  it('rejects a new password equal to the current one before calling the API', async () => {
    const api = fakeApi({ 'GET /me': { status: 200, body: user } })
    const { user: actions } = renderApp('/conta')

    await actions.click(await screen.findByRole('tab', { name: 'Segurança' }))
    await actions.type(screen.getByLabelText('Senha atual'), 'senha12345')
    await actions.type(screen.getByLabelText('Nova senha'), 'senha12345')
    await actions.type(screen.getByLabelText('Confirme a nova senha'), 'senha12345')
    await actions.click(screen.getByRole('button', { name: 'Trocar senha' }))

    expect(screen.getByText('A nova senha precisa ser diferente da atual.')).toBeInTheDocument()
    expect(api.calls('POST', '/me/password')).toHaveLength(0)
  })

  it('explains that a Google-only account has no password to change', async () => {
    fakeApi({ 'GET /me': { status: 200, body: { ...user, has_password: false, has_google: true } } })
    const { user: actions } = renderApp('/conta')

    await actions.click(await screen.findByRole('tab', { name: 'Segurança' }))
    expect(screen.getByText(/Sua conta entra com o Google/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Senha atual')).not.toBeInTheDocument()
  })
})
