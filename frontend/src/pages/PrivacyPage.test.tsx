import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { fakeApi, signedOut } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

describe('PrivacyPage', () => {
  it('is public and gives the contact for data requests', async () => {
    fakeApi(signedOut)
    renderApp('/privacidade')

    expect(await screen.findByRole('heading', { level: 1, name: 'Política de privacidade' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'privacidade@shortbit.com.br' })).toHaveAttribute('href', 'mailto:privacidade@shortbit.com.br')
    expect(screen.getByTestId('current-path')).toHaveTextContent('/privacidade')
  })

  it('is linked from the footer', async () => {
    fakeApi(signedOut)
    renderApp('/')

    const footer = await screen.findByRole('contentinfo')
    expect(within(footer).getByRole('link', { name: 'Política de privacidade' })).toHaveAttribute('href', '/privacidade')
  })
})
