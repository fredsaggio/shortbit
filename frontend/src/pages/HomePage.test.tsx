import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

describe('HomePage', () => {
  it('does not advertise plans that do not exist', async () => {
    fakeApi(signedOut)
    renderApp('/')

    expect(await screen.findByRole('heading', { level: 1 })).toHaveTextContent('Links curtos que mostram quem clicou')
    expect(screen.queryByText(/plano grátis/i)).not.toBeInTheDocument()
  })

  it('sends a visitor without an account to sign up, keeping the URL for later', async () => {
    const api = fakeApi(signedOut)
    const { user: actions } = renderApp('/')

    await actions.type(await screen.findByLabelText('URL para encurtar'), 'https://example.com/um-link-bem-longo')
    await actions.click(screen.getByRole('button', { name: 'Encurtar' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/cadastro'))
    expect(sessionStorage.getItem('shortbit:pending-url')).toBe('https://example.com/um-link-bem-longo')
    expect(api.calls('POST', '/urls')).toHaveLength(0)
  })
})
