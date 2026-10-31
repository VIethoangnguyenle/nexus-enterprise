import { useMemo } from 'react'
import QRCode from 'qrcode'

/**
 * A QR code drawn as one SVG path. Error correction H leaves room for the logo
 * tile the caller lays over the centre. The light modules and the quiet zone are
 * true white in both themes: a camera needs a bright margin around the code
 * (documented colour exemption, design/mockups/qr-login.html §1 note 2).
 */
export function QrCode({ value, label, className = '' }: { value: string; label: string; className?: string }) {
  const { size, path } = useMemo(() => {
    const qr = QRCode.create(value, { errorCorrectionLevel: 'H' })
    const n = qr.modules.size
    let d = ''
    for (let y = 0; y < n; y++) {
      for (let x = 0; x < n; x++) {
        if (qr.modules.get(y, x)) d += `M${x} ${y}h1v1h-1z`
      }
    }
    return { size: n, path: d }
  }, [value])

  return (
    <svg
      role="img"
      aria-label={label}
      viewBox={`0 0 ${size} ${size}`}
      shapeRendering="crispEdges"
      className={`block w-full h-full ${className}`}
    >
      <rect width={size} height={size} fill="#fff" />
      <path d={path} fill="#111" />
    </svg>
  )
}
