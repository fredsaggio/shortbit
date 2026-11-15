import { describe, expect, it } from 'vitest'
import { qrCodeSVG, qrCodeTarget, svgDataURL } from './qrCode.ts'

describe('qrCode', () => {
  it('draws the code in navy on white', async () => {
    const svg = await qrCodeSVG('https://shortbit.com.br/Ab3dX9')

    expect(svg).toMatch(/^<svg/)
    expect(svg).toContain('#031f39')
    expect(svg).toContain('#ffffff')
  })

  it('embeds the SVG as a data URL the page can show and download', () => {
    expect(svgDataURL('<svg a="1"></svg>')).toBe('data:image/svg+xml;charset=utf-8,%3Csvg%20a%3D%221%22%3E%3C%2Fsvg%3E')
  })

  it('points the code at the short link marked as a scan', () => {
    expect(qrCodeTarget('https://shortbit.com.br/Ab3dX9')).toBe('https://shortbit.com.br/Ab3dX9?qr=1')
  })
})
