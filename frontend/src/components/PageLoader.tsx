export function PageLoader({ label = 'Carregando…' }: { label?: string }) {
  return (
    <div className="page-loader" role="status">
      <span className="spinner spinner--large" aria-hidden="true" />
      <span>{label}</span>
    </div>
  )
}
