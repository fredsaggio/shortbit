import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ShortLink } from '../api/types.ts'
import { savePendingLink } from '../pendingLink.ts'
import { fakeApi, signedOut, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

function link(code: string, overrides: Partial<ShortLink> = {}): ShortLink {
  return {
    short_code: code,
    original_url: `https://example.com/${code}`,
    visibility: 'public',
    click_count: 0,
    created_at: '2026-11-14T12:00:00Z',
    updated_at: '2026-11-14T12:00:00Z',
    ...overrides,
  }
}

const signedIn = { 'GET /me': { status: 200, body: user } }

describe('DashboardPage', () => {
  it('sends a visitor without a session to the login page', async () => {
    fakeApi(signedOut)
    renderApp('/painel')

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/entrar'))
  })

  it('lists links and loads the next page', async () => {
    fakeApi({
      ...signedIn,
      'GET /urls': {
        status: 200,
        body: { urls: [link('Ab3dX9', { click_count: 1234 }), link('Qz7Rt2', { visibility: 'private' })], next_cursor: 'cursor-2' },
      },
      'GET /urls?cursor=cursor-2': { status: 200, body: { urls: [link('Zx9Wq1')], next_cursor: null } },
    })
    const { user: actions } = renderApp('/painel')

    const list = await screen.findByRole('list', { name: 'Seus links' })
    const items = within(list).getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(within(items[0]).getByText('1.234')).toBeInTheDocument()
    expect(within(items[1]).getByText('Privado')).toBeInTheDocument()

    await actions.click(screen.getByRole('button', { name: 'Carregar mais' }))

    expect(await screen.findByText('https://example.com/Zx9Wq1')).toBeInTheDocument()
    expect(within(list).getAllByRole('listitem')).toHaveLength(3)
    expect(screen.queryByRole('button', { name: 'Carregar mais' })).not.toBeInTheDocument()
  })

  it('creates a private link with its password and refreshes the list', async () => {
    let created = false
    const api = fakeApi({
      ...signedIn,
      'GET /urls': () => ({ status: 200, body: { urls: created ? [link('Ab3dX9', { visibility: 'private' })] : [], next_cursor: null } }),
      'POST /urls': () => {
        created = true
        return { status: 201, body: { short_code: 'Ab3dX9', short_url: 'http://localhost:8080/Ab3dX9' } }
      },
    })
    const { user: actions } = renderApp('/painel')

    expect(await screen.findByText(/ainda não criou nenhum link/)).toBeInTheDocument()

    await actions.type(screen.getByLabelText('URL de destino'), 'https://example.com/segredo')
    await actions.click(screen.getByRole('radio', { name: 'Privado' }))
    await actions.type(screen.getByLabelText('Senha do link'), 'senha-do-link')
    await actions.click(screen.getByRole('button', { name: 'Criar link curto' }))

    expect(await screen.findByRole('link', { name: 'http://localhost:8080/Ab3dX9' })).toBeInTheDocument()
    expect(api.calls('POST', '/urls')[0].body).toEqual({
      url: 'https://example.com/segredo',
      visibility: 'private',
      password: 'senha-do-link',
    })
    expect(await screen.findByText('https://example.com/Ab3dX9')).toBeInTheDocument()
  })

  it('sends a public link without a password field', async () => {
    const api = fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
      'POST /urls': { status: 201, body: { short_code: 'Ab3dX9', short_url: 'http://localhost:8080/Ab3dX9' } },
    })
    const { user: actions } = renderApp('/painel')

    await actions.type(await screen.findByLabelText('URL de destino'), 'https://example.com')
    await actions.click(screen.getByRole('button', { name: 'Criar link curto' }))

    await waitFor(() => expect(api.calls('POST', '/urls')).toHaveLength(1))
    expect(api.calls('POST', '/urls')[0].body).toEqual({ url: 'https://example.com', visibility: 'public' })
  })

  it('validates the URL and the link password before creating', async () => {
    const api = fakeApi({ ...signedIn, 'GET /urls': { status: 200, body: { urls: [], next_cursor: null } } })
    const { user: actions } = renderApp('/painel')

    await actions.type(await screen.findByLabelText('URL de destino'), 'exemplo.com')
    await actions.click(screen.getByRole('radio', { name: 'Privado' }))
    await actions.type(screen.getByLabelText('Senha do link'), 'curta')
    await actions.click(screen.getByRole('button', { name: 'Criar link curto' }))

    expect(screen.getByText('Digite uma URL completa, começando com http:// ou https://.')).toBeInTheDocument()
    expect(screen.getByText('A senha do link precisa ter pelo menos 8 caracteres.')).toBeInTheDocument()
    expect(api.calls('POST', '/urls')).toHaveLength(0)
  })

  it('shortens the URL typed on the home page before login, once', async () => {
    savePendingLink({ url: 'https://example.com/da-home', visibility: 'public' })
    const api = fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
      'POST /urls': { status: 201, body: { short_code: 'Ab3dX9', short_url: 'http://localhost:8080/Ab3dX9' } },
    })
    renderApp('/painel')

    const result = await screen.findByRole('link', { name: 'http://localhost:8080/Ab3dX9' })
    expect(result).toBeInTheDocument()
    expect(api.calls('POST', '/urls')).toHaveLength(1)
    expect(api.calls('POST', '/urls')[0].body).toEqual({ url: 'https://example.com/da-home', visibility: 'public' })
    expect(sessionStorage.getItem('shortbit:pending-link')).toBeNull()
    expect(screen.getByLabelText('URL de destino')).toHaveValue('')
  })

  it('keeps the home page URL in the form when creating it fails', async () => {
    savePendingLink({ url: 'https://example.com/da-home', visibility: 'public' })
    fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
      'POST /urls': { status: 400, body: 'url inválida' },
    })
    renderApp('/painel')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByLabelText('URL de destino')).toHaveValue('https://example.com/da-home')
  })

  it('creates a private link from the home page with the password kept in memory', async () => {
    savePendingLink({ url: 'https://example.com/segredo', visibility: 'private', password: 'senha-do-link' })
    expect(sessionStorage.getItem('shortbit:pending-link')).not.toContain('senha-do-link')
    const api = fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
      'POST /urls': { status: 201, body: { short_code: 'Qz7Rt2', short_url: 'http://localhost:8080/Qz7Rt2' } },
    })
    renderApp('/painel')

    expect(await screen.findByRole('link', { name: 'http://localhost:8080/Qz7Rt2' })).toBeInTheDocument()
    expect(api.calls('POST', '/urls')).toHaveLength(1)
    expect(api.calls('POST', '/urls')[0].body).toEqual({
      url: 'https://example.com/segredo',
      visibility: 'private',
      password: 'senha-do-link',
    })
  })

  it('asks again for a private link password lost to a page load, without creating the link', async () => {
    // What survives a full page load, such as the Google sign-in: storage
    // only, never the password.
    sessionStorage.setItem('shortbit:pending-link', JSON.stringify({ url: 'https://example.com/segredo', visibility: 'private' }))
    const api = fakeApi({ ...signedIn, 'GET /urls': { status: 200, body: { urls: [], next_cursor: null } } })
    renderApp('/painel')

    expect(await screen.findByText(/Digite de novo a senha do link/)).toBeInTheDocument()
    expect(screen.getByLabelText('URL de destino')).toHaveValue('https://example.com/segredo')
    expect(screen.getByRole('radio', { name: 'Privado' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByLabelText('Senha do link')).toHaveValue('')
    expect(api.calls('POST', '/urls')).toHaveLength(0)
    expect(sessionStorage.getItem('shortbit:pending-link')).toBeNull()
  })

  it('rates the link password on a strength bar instead of a checklist', async () => {
    fakeApi({ ...signedIn, 'GET /urls': { status: 200, body: { urls: [], next_cursor: null } } })
    const { user: actions } = renderApp('/painel')

    await actions.click(await screen.findByRole('radio', { name: 'Privado' }))
    const field = screen.getByLabelText('Senha do link')
    expect(screen.queryByRole('list', { name: 'Requisitos da senha' })).not.toBeInTheDocument()

    await actions.type(field, 'curta')
    expect(screen.getByText('Curta demais · mínimo de 8 caracteres')).toBeInTheDocument()

    await actions.clear(field)
    await actions.type(field, 'password')
    expect(await screen.findByText('Muito fraca')).toBeInTheDocument()

    await actions.clear(field)
    await actions.type(field, 'cavalo correto bateria grampeador')
    expect(await screen.findByText('Muito forte', {}, { timeout: 5000 })).toBeInTheDocument()
    expect(field).toHaveAccessibleDescription(/Força da senha: Muito forte/)
  })

  it('deletes a link from its menu after confirming', async () => {
    const api = fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [link('Ab3dX9'), link('Qz7Rt2')], next_cursor: null } },
      'DELETE /urls/Ab3dX9': { status: 204 },
    })
    const { user: actions } = renderApp('/painel')

    const list = await screen.findByRole('list', { name: 'Seus links' })
    await actions.click(within(list).getByRole('button', { name: 'Ações do link localhost:8080/Ab3dX9' }))
    await actions.click(await screen.findByRole('menuitem', { name: 'Excluir' }))
    const dialog = await screen.findByRole('alertdialog')
    await actions.click(within(dialog).getByRole('button', { name: 'Excluir link' }))

    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(1))
    expect(within(list).queryByText('localhost:8080/Ab3dX9')).not.toBeInTheDocument()
    expect(api.calls('DELETE', '/urls/Ab3dX9')).toHaveLength(1)
  })

  it('opens the edit form from the menu', async () => {
    fakeApi({
      ...signedIn,
      'GET /urls': { status: 200, body: { urls: [link('Ab3dX9')], next_cursor: null } },
      'GET /urls/Ab3dX9': { status: 200, body: link('Ab3dX9') },
    })
    const { user: actions } = renderApp('/painel')

    await actions.click(await screen.findByRole('button', { name: 'Ações do link localhost:8080/Ab3dX9' }))
    await actions.click(await screen.findByRole('menuitem', { name: 'Editar' }))

    expect(await screen.findByRole('form', { name: 'Editar link' })).toBeInTheDocument()
    expect(screen.getByTestId('current-path')).toHaveTextContent('/painel/links/Ab3dX9')
  })

  it('opens the QR code of a link from its menu', async () => {
    fakeApi({ ...signedIn, 'GET /urls': { status: 200, body: { urls: [link('Ab3dX9')], next_cursor: null } } })
    const { user: actions } = renderApp('/painel')

    await actions.click(await screen.findByRole('button', { name: 'Ações do link localhost:8080/Ab3dX9' }))
    await actions.click(await screen.findByRole('menuitem', { name: 'QR code' }))

    const dialog = await screen.findByRole('dialog', { name: 'QR code do link' })
    expect(await within(dialog).findByRole('img', { name: 'QR code de localhost:8080/Ab3dX9' })).toBeInTheDocument()
  })
})
