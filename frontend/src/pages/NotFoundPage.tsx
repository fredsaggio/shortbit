import { Link } from 'react-router'

export function NotFoundPage() {
  return (
    <section className="container not-found">
      <h1>Página não encontrada</h1>
      <p>O endereço que você abriu não existe aqui.</p>
      <Link to="/" className="button button--primary">
        Voltar para o início
      </Link>
    </section>
  )
}
