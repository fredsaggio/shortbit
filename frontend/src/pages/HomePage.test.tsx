import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

describe('HomePage', () => {
  it('does not advertise plans that do not exist', async () => {
    fakeApi(signedOut)
    renderApp('/')

    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Encurte, proteja e conte cada clique')
    expect(screen.queryByText(/plano grátis/i)).not.toBeInTheDocument()
  })

  it('sends a visitor without an account to sign up, keeping the URL for later', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/')

    await actions.type(await screen.findByLabelText('URL para encurtar'), 'https://example.com/um-link-bem-longo')
    await actions.click(screen.getByRole('button', { name: 'Encurtar' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/cadastro'))
    expect(JSON.parse(sessionStorage.getItem('shortbit:pending-link') ?? '')).toEqual({
      url: 'https://example.com/um-link-bem-longo',
      visibility: 'public',
    })
    expect(api.calls('POST', '/urls')).toHaveLength(0)
  })

  it('lets a visitor ask for a password-protected link without storing the password', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/')

    await actions.type(await screen.findByLabelText('URL para encurtar'), 'https://example.com/segredo')
    await actions.click(screen.getByRole('radio', { name: 'Privado' }))
    await actions.type(screen.getByLabelText('Senha do link'), 'senha-do-link')
    await actions.click(screen.getByRole('button', { name: 'Encurtar' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/cadastro'))
    const stored = sessionStorage.getItem('shortbit:pending-link') ?? ''
    expect(JSON.parse(stored)).toEqual({ url: 'https://example.com/segredo', visibility: 'private' })
    expect(stored).not.toContain('senha-do-link')
    expect(api.calls('POST', '/urls')).toHaveLength(0)
  })

  it('checks the link password before leaving the home page', async () => {
    fakeApi(signedOut)
    const { user: actions } = renderApp('/')

    await actions.type(await screen.findByLabelText('URL para encurtar'), 'https://example.com/segredo')
    await actions.click(screen.getByRole('radio', { name: 'Privado' }))
    await actions.type(screen.getByLabelText('Senha do link'), 'curta')
    await actions.click(screen.getByRole('button', { name: 'Encurtar' }))

    expect(await screen.findByText('A senha do link precisa ter pelo menos 8 caracteres.')).toBeInTheDocument()
    expect(screen.getByTestId('current-path')).toHaveTextContent('/')
    expect(sessionStorage.getItem('shortbit:pending-link')).toBeNull()
  })
})
