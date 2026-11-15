import type { ReactNode } from 'react'

export const privacyContactEmail = 'privacidade@shortbit.com.br'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-3">
      <h2 className="text-xl font-bold">{title}</h2>
      <div className="grid gap-3 leading-relaxed text-muted-foreground">{children}</div>
    </section>
  )
}

// Everything here describes what the code actually does; update it together
// with any change to what is stored, logged or shared.
export function PrivacyPage() {
  return (
    <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6 sm:py-14">
      <header>
        <h1 className="text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">Política de privacidade</h1>
        <p className="mt-2 text-muted-foreground">Atualizada em 14 de novembro de 2026.</p>
      </header>

      <div className="mt-8 grid gap-8 rounded-3xl border bg-card p-6 shadow-card sm:p-10">
        <p className="leading-relaxed text-muted-foreground">
          O shortbit encurta links e conta quantas vezes eles são abertos. Esta página explica quais dados guardamos, para quê e
          como você pode pedir acesso ou exclusão, de acordo com a Lei Geral de Proteção de Dados (LGPD).
        </p>

        <Section title="O que guardamos sobre a sua conta">
          <ul className="grid list-disc gap-2 pl-5">
            <li>Email e nome completo.</li>
            <li>
              Sua senha, guardada só como hash (Argon2id). Ninguém, nem nós, consegue ver a senha que você digitou.
            </li>
            <li>
              Se você entra com o Google: o identificador da sua conta Google, seu email e seu nome. Não guardamos sua foto nem
              os tokens de acesso do Google.
            </li>
            <li>Sessões de login, guardadas só como hash, que deixam de valer quando expiram ou quando você sai.</li>
          </ul>
        </Section>

        <Section title="O que guardamos sobre os seus links">
          <ul className="grid list-disc gap-2 pl-5">
            <li>A URL de destino, o código curto, a data de criação e se o link é público ou privado.</li>
            <li>Para links privados, a senha do link, guardada só como hash.</li>
            <li>Quantas vezes cada link foi aberto.</li>
          </ul>
        </Section>

        <Section title="Quem clica nos links">
          <p>
            Não guardamos nada sobre quem abre um link: nem IP, nem navegador, nem localização. Cada clique só soma um na
            contagem do link. O endereço IP é usado por alguns minutos, apenas na memória do servidor, para limitar abusos (como
            tentativas repetidas de senha), e nunca é gravado.
          </p>
        </Section>

        <Section title="Cookies">
          <p>
            Usamos apenas cookies essenciais: para manter você conectado, para concluir o cadastro, a recuperação de senha e o
            login com Google, e para lembrar que você já digitou a senha de um link privado. Não usamos cookies de publicidade
            nem de rastreamento, e não há ferramentas de análise de terceiros.
          </p>
        </Section>

        <Section title="Com quem os dados são compartilhados">
          <p>Não vendemos nem compartilhamos dados para publicidade. Para funcionar, o shortbit usa estes serviços:</p>
          <ul className="grid list-disc gap-2 pl-5">
            <li>Render: hospeda a aplicação.</li>
            <li>Neon: guarda o banco de dados.</li>
            <li>Upstash e CloudAMQP: guardam temporariamente os destinos dos links mais acessados e a fila de cliques.</li>
            <li>Resend: envia os emails com códigos de confirmação e de recuperação de senha.</li>
            <li>Google: só quando você escolhe entrar com o Google.</li>
          </ul>
          <p>
            Render, Neon, Upstash e CloudAMQP ficam nos Estados Unidos, então os dados são transferidos para fora do Brasil. Os
            provedores de hospedagem podem registrar dados técnicos de acesso, como o IP, para a segurança da própria
            infraestrutura.
          </p>
        </Section>

        <Section title="Por quanto tempo">
          <p>
            Os dados da conta e dos links ficam guardados enquanto a conta existir. Tentativas de cadastro e de recuperação de
            senha expiram em minutos, e os registros internos de cliques já contados são apagados em 7 dias.
          </p>
        </Section>

        <Section title="Seus direitos">
          <p>
            Você pode pedir a confirmação de que tratamos seus dados, uma cópia deles, a correção ou a exclusão da sua conta e
            dos seus links. Seu nome e sua senha você mesmo altera em Minha conta. Para os outros pedidos, escreva para{' '}
            <a href={`mailto:${privacyContactEmail}`} className="font-semibold text-action underline-offset-4 hover:underline">
              {privacyContactEmail}
            </a>{' '}
            usando o email da sua conta.
          </p>
        </Section>
      </div>
    </div>
  )
}
