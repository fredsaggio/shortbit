import QRCode from 'qrcode'

// The code holds the short link, never the destination: each scan counts as a
// click, and a printed code keeps working when the destination changes.
// Level M survives smudges and folds on print; the four-module margin is the
// quiet zone readers need.
const options = {
  errorCorrectionLevel: 'M',
  margin: 4,
  color: { dark: '#031f39', light: '#ffffff' },
} as const

// What the QR code holds: the short link plus ?qr=1, which tells the API the
// click came from a scan (ADR 0005). The destination does not change.
export function qrCodeTarget(shortURL: string): string {
  return `${shortURL}?qr=1`
}

export function qrCodeSVG(text: string): Promise<string> {
  return QRCode.toString(text, { ...options, type: 'svg' })
}

// 1024 px keeps the PNG sharp in chats and on most prints.
export function qrCodePNG(text: string): Promise<string> {
  return QRCode.toDataURL(text, { ...options, type: 'image/png', width: 1024 })
}

export function svgDataURL(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

export function downloadFile(href: string, filename: string) {
  const anchor = document.createElement('a')
  anchor.href = href
  anchor.download = filename
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
}
