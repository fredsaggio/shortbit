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

  it('shows the unique visitors and the robot previews of the period', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-13': { link: 2, visitors: 2, robots: 1 }, '2026-11-14': { link: 3, visitors: 2, robots: 4 } }),
      },
    })
    const { chart } = await openChart()

    const totals = await within(chart).findByRole('list', { name: 'Total no período' })
    expect(within(totals).getByText('Visitantes únicos').nextSibling).toHaveTextContent('4')
    expect(within(totals).getByText('Prévias de robôs').nextSibling).toHaveTextContent('5')
  })

  it('lists the clicks by country, most first, with the unknown ones last', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-14': { link: 10 } }, [
          { country: 'BR', clicks: 6 },
          { country: 'PT', clicks: 3 },
        ]),
      },
    })
    const { chart } = await openChart()

    const countries = await within(chart).findByRole('list', { name: 'Países' })
    const items = within(countries).getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(items[0]).toHaveTextContent(/Brasil\s*6 cliques/)
    expect(items[1]).toHaveTextContent(/Portugal\s*3 cliques/)
    expect(items[2]).toHaveTextContent(/País não identificado\s*1 clique$/)
    expect(within(chart).queryByRole('link', { name: 'DB-IP' })).not.toBeInTheDocument()
    // The country database's license asks for this link on the page.
    expect(within(screen.getByRole('contentinfo')).getByRole('link', { name: 'DB-IP' })).toHaveAttribute('href', 'https://db-ip.com')
  })

  it('adds up the countries past the eighth in one row', async () => {
    const codes = ['BR', 'PT', 'US', 'AR', 'MX', 'FR', 'DE', 'ES', 'IT', 'JP']
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-14': { link: 55 } }, codes.map((country, i) => ({ country, clicks: 10 - i }))),
      },
    })
    const { chart } = await openChart()

    const countries = await within(chart).findByRole('list', { name: 'Países' })
    const items = within(countries).getAllByRole('listitem')
    expect(items).toHaveLength(9)
    expect(items[8]).toHaveTextContent(/Outros 2 países\s*3 cliques/)
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
        body: dailyClicks(30, { '2026-11-13': { link: 2, visitors: 1 }, '2026-11-14': { link: 1, qr: 4, visitors: 4, robots: 2 } }),
      },
    })
    const { chart, user: actions } = await openChart()

    const plot = await within(chart).findByRole('group', { name: /Gráfico de cliques por dia/ })
    await actions.click(plot)
    await actions.keyboard('{End}')
    expect(within(chart).getByText(/^14 de novembro: 5 cliques, 1 pelo link e 4 pelo QR code, 4 visitantes únicos, 2 prévias de robôs$/)).toBeInTheDocument()

    await actions.keyboard('{ArrowLeft}')
    expect(within(chart).getByText(/^13 de novembro: 2 cliques, 2 pelo link e 0 pelo QR code, 1 visitante único$/)).toBeInTheDocument()
  })

  it('lists the days with clicks in a table, newest first', async () => {
    fakeApi({
      ...base,
      'GET /urls/Ab3dX9/clicks?days=30': {
        status: 200,
        body: dailyClicks(30, { '2026-11-02': { qr: 1, visitors: 1 }, '2026-11-08': { robots: 3 }, '2026-11-14': { link: 1, qr: 4, visitors: 3, robots: 1 } }),
      },
    })
    const { chart, user: actions } = await openChart()

    await actions.click(await within(chart).findByText('Ver os números em tabela'))
    const rows = within(within(chart).getByRole('table')).getAllByRole('row').slice(1)

    // Day, by link, by QR code, total, unique visitors and robot previews; a
    // day with robot previews only is listed too.
    expect(rows).toHaveLength(3)
    expect(rows[0]).toHaveTextContent('14 de novembro14531')
    expect(rows[1]).toHaveTextContent('8 de novembro00003')
    expect(rows[2]).toHaveTextContent('2 de novembro01110')
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
