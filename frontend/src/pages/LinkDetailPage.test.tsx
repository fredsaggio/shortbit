import { screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ShortLink } from '../api/types.ts'
import { fakeApi, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

const publicLink: ShortLink = {
  short_code: 'Ab3dX9',
  original_url: 'https://example.com/antigo',
  visibility: 'public',
  click_count: 4,
  created_at: '2026-11-14T12:00:00Z',
  updated_at: '2026-11-14T12:00:00Z',
}

const signedIn = { 'GET /me': { status: 200, body: user } }

describe('LinkDetailPage', () => {
  it('edits the destination, sending only what changed', async () => {
    const api = fakeApi({
      ...signedIn,
      'GET /urls/Ab3dX9': { status: 200, body: publicLink },
      'PATCH /urls/Ab3dX9': { status: 200, body: { ...publicLink, original_url: 'https://example.com/novo' } },
    })
    const { user: actions } = renderApp('/painel/links/Ab3dX9')

    await actions.click(await screen.findByRole('button', { name: 'Editar' }))
    const destination = screen.getByLabelText('Destino')
    await actions.clear(destination)
    await actions.type(destination, 'https://example.com/novo')
    await actions.click(screen.getByRole('button', { name: 'Salvar alterações' }))

    expect(await screen.findByRole('link', { name: 'https://example.com/novo' })).toBeInTheDocument()
    expect(api.calls('PATCH', '/urls/Ab3dX9')[0].body).toEqual({ url: 'https://example.com/novo' })
    expect(screen.queryByRole('form', { name: 'Editar link' })).not.toBeInTheDocument()
  })

  it('asks for a password before making a public link private', async () => {
    const api = fakeApi({ ...signedIn, 'GET /urls/Ab3dX9': { status: 200, body: publicLink } })
    const { user: actions } = renderApp('/painel/links/Ab3dX9?editar=1')

    const form = await screen.findByRole('form', { name: 'Editar link' })
    await actions.click(within(form).getByRole('radio', { name: 'Privado' }))
    await actions.click(within(form).getByRole('button', { name: 'Salvar alterações' }))

    expect(await screen.findByText('Defina a senha que vai proteger o link.')).toBeInTheDocument()
    expect(api.calls('PATCH', '/urls/Ab3dX9')).toHaveLength(0)
  })

  it('keeps the current password of a private link when the field is left blank', async () => {
    const privateLink = { ...publicLink, visibility: 'private' as const }
    const api = fakeApi({
      ...signedIn,
      'GET /urls/Ab3dX9': { status: 200, body: privateLink },
      'PATCH /urls/Ab3dX9': { status: 200, body: { ...privateLink, original_url: 'https://example.com/novo' } },
    })
    const { user: actions } = renderApp('/painel/links/Ab3dX9?editar=1')

    const destination = await screen.findByLabelText('Destino')
    expect(screen.getByLabelText('Nova senha do link')).toHaveValue('')
    await actions.clear(destination)
    await actions.type(destination, 'https://example.com/novo')
    await actions.click(screen.getByRole('button', { name: 'Salvar alterações' }))

    await waitFor(() => expect(api.calls('PATCH', '/urls/Ab3dX9')).toHaveLength(1))
    expect(api.calls('PATCH', '/urls/Ab3dX9')[0].body).toEqual({ url: 'https://example.com/novo' })
  })

  it('deletes the link after confirming and goes back to the dashboard', async () => {
    const api = fakeApi({
      ...signedIn,
      'GET /urls/Ab3dX9': { status: 200, body: publicLink },
      'DELETE /urls/Ab3dX9': { status: 204 },
      'GET /urls': { status: 200, body: { urls: [], next_cursor: null } },
    })
    const { user: actions } = renderApp('/painel/links/Ab3dX9')

    await actions.click(await screen.findByRole('button', { name: 'Excluir' }))
    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Excluir este link?')).toBeInTheDocument()
    expect(api.calls('DELETE', '/urls/Ab3dX9')).toHaveLength(0)

    await actions.click(within(dialog).getByRole('button', { name: 'Excluir link' }))

    await waitFor(() => expect(screen.getByTestId('current-path')).toHaveTextContent('/painel'))
    expect(api.calls('DELETE', '/urls/Ab3dX9')).toHaveLength(1)
  })

  it('shows the error inside the dialog when deleting fails', async () => {
    fakeApi({
      ...signedIn,
      'GET /urls/Ab3dX9': { status: 200, body: publicLink },
      'DELETE /urls/Ab3dX9': { status: 500, body: 'erro interno do servidor' },
    })
    const { user: actions } = renderApp('/painel/links/Ab3dX9')

    await actions.click(await screen.findByRole('button', { name: 'Excluir' }))
    const dialog = await screen.findByRole('alertdialog')
    await actions.click(within(dialog).getByRole('button', { name: 'Excluir link' }))

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Algo deu errado do nosso lado')
    expect(screen.getByTestId('current-path')).toHaveTextContent('/painel/links/Ab3dX9')
  })

  describe('QR code', () => {
    afterEach(() => vi.restoreAllMocks())

    it('shows the code of the short link and downloads it as PNG and SVG', async () => {
      const downloads: { href: string; download: string }[] = []
      vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
        downloads.push({ href: this.href, download: this.download })
      })
      fakeApi({ ...signedIn, 'GET /urls/Ab3dX9': { status: 200, body: publicLink } })
      const { user: actions } = renderApp('/painel/links/Ab3dX9')

      await actions.click(await screen.findByRole('button', { name: 'QR code' }))
      const dialog = await screen.findByRole('dialog', { name: 'QR code do link' })
      const image = await within(dialog).findByRole('img', { name: 'QR code de localhost:8080/Ab3dX9' })
      expect(image.getAttribute('src')).toMatch(/^data:image\/svg\+xml/)

      await actions.click(within(dialog).getByRole('button', { name: 'Baixar SVG' }))
      await actions.click(within(dialog).getByRole('button', { name: 'Baixar PNG' }))

      await waitFor(() => expect(downloads).toHaveLength(2))
      expect(downloads[0]).toMatchObject({ download: 'qrcode-Ab3dX9.svg' })
      expect(downloads[0].href).toMatch(/^data:image\/svg\+xml/)
      expect(downloads[1]).toMatchObject({ download: 'qrcode-Ab3dX9.png' })
      expect(downloads[1].href).toMatch(/^data:image\/png;base64,/)
    })
  })
})
