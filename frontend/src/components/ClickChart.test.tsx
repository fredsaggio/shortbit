import { screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { ShortLink } from '../api/types.ts'
import { dailyClicks, fakeApi, user } from '../test/fakeApi.ts'
import { renderApp } from '../test/renderApp.tsx'

const link: ShortLink = {
  short_code: 'Ab3dX9',
  original_url: 'https://example.com/destino',
  visibility: 'public',
  click_count: 40,
  created_at: '2026-11-01T12:00:00Z',
  updated_at: '2026-11-01T12:00:00Z',
}

const base = {
  'GET /me': { status: 200, body: user },
  'GET /urls/Ab3dX9': { status: 200, body: link },
}

async function openChart() {
  const view = renderApp('/painel/links/Ab3dX9')
  const chart = await screen.findByRole('region', { name: 'Cliques por dia' })
  return { ...view, chart }
}

describe('ClickChart', () => {
  it('shows the totals by link and by QR code for the last 30 days', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-13': { link: 2 }, '2026-11-14': { link: 1, qr: 4 } }),
      },
    })
    const { chart } = await openChart()

    const totals = await within(chart).findByRole('list', { name: 'Total no período' })
    expect(within(totals).getByText('Pelo link').nextSibling).toHaveTextContent('3')
    expect(within(totals).getByText('Pelo QR code').nextSibling).toHaveTextContent('4')
    expect(within(chart).getByRole('radio', { name: '30 dias' })).toBeChecked()
  })

  it('switches the period', async () => {
    const api = fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': { status: 200, body: dailyClicks(30) },
      'GET /urls/Ab3dX9/clicks?days=7': { status: 200, body: dailyClicks(7, { '2026-11-14': { qr: 2 } }) },
    })
    const { chart, user: actions } = await openChart()

    await within(chart).findByText('Nenhum clique neste período.', { selector: 'p.absolute' })
    await actions.click(within(chart).getByRole('radio', { name: '7 dias' }))

    const totals = within(chart).getByRole('list', { name: 'Total no período' })
    await waitFor(() => expect(within(totals).getByText('Pelo QR code').nextSibling).toHaveTextContent('2'))
    expect(api.calls('GET', '/urls/Ab3dX9/clicks?days=7')).toHaveLength(1)
  })

  it('reads each day aloud with the arrow keys', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-13': { link: 2 }, '2026-11-14': { link: 1, qr: 4 } }),
      },
    })
    const { chart, user: actions } = await openChart()

    const plot = await within(chart).findByRole('group', { name: /Gráfico de cliques por dia/ })
    await actions.click(plot)
    await actions.keyboard('{End}')
    expect(within(chart).getByText(/^14 de novembro: 5 cliques, 1 pelo link e 4 pelo QR code$/)).toBeInTheDocument()

    await actions.keyboard('{ArrowLeft}')
    expect(within(chart).getByText(/^13 de novembro: 2 cliques, 2 pelo link e 0 pelo QR code$/)).toBeInTheDocument()
  })

  it('lists the days with clicks in a table, newest first', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-02': { qr: 1 }, '2026-11-14': { link: 1, qr: 4 } }),
      },
    })
    const { chart, user: actions } = await openChart()

    await actions.click(await within(chart).findByText('Ver os números em tabela'))
    const rows = within(within(chart).getByRole('table')).getAllByRole('row').slice(1)

    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('14 de novembro145')
    expect(rows[1]).toHaveTextContent('2 de novembro011')
  })

  it('asks for the clicks again when the page refreshes', async () => {
    const api = fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': { status: 200, body: dailyClicks(30) },
    })
    const { chart, user: actions } = await openChart()
    await within(chart).findByRole('list', { name: 'Total no período' })
    const before = api.calls('GET', '/urls/Ab3dX9/clicks?days=30').length

    await actions.click(screen.getByRole('button', { name: 'Atualizar' }))

    await waitFor(() => expect(api.calls('GET', '/urls/Ab3dX9/clicks?days=30')).toHaveLength(before + 1))
  })

  it('shows the error when the clicks do not load', async () => {
    fakeApi({ ...base, 'GET /urls/Ab3dX9/clicks?days=30': { status: 500, body: 'erro interno do servidor' } })
    const { chart } = await openChart()

    expect(await within(chart).findByRole('alert')).toHaveTextContent('Algo deu errado do nosso lado')
  })
})
