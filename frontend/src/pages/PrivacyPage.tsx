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

// Plain language for people who are not programmers: no vendor or technology
// names. It still must match what the code does; update it with any change to
// what is stored, logged or shared.
export function PrivacyPage() {
  return (
    <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6 sm:py-14">
      <header>
        <h1 className="text-[clamp(1.875rem,4vw,2.5rem)] font-extrabold">Política de privacidade</h1>
        <p className="mt-2 text-muted-foreground">Atualizada em 14 de novembro de 2026.</p>
      </header>

      <div className="mt-8 grid gap-8 rounded-3xl border bg-card p-6 shadow-card sm:p-10">
        <p className="leading-relaxed text-muted-foreground">
          O shortbit encurta links e conta quantas vezes eles são abertos. Aqui você vê, em poucas palavras, quais dados
          guardamos, para que servem e como pedir uma cópia ou a exclusão deles, como garante a Lei Geral de Proteção de Dados
          (LGPD).
        </p>

        <Section title="O que guardamos sobre você">
          <ul className="grid list-disc gap-2 pl-5">
            <li>Seu email e seu nome.</li>
            <li>Sua senha, guardada de um jeito que ninguém consegue lê-la, nem a nossa equipe.</li>
            <li>
              Se você entra com o Google, recebemos dele seu nome, seu email e uma identificação da sua conta. Não guardamos sua
              foto.
            </li>
          </ul>
        </Section>

        <Section title="O que guardamos sobre os seus links">
          <ul className="grid list-disc gap-2 pl-5">
            <li>O endereço de destino, o link curto, a data de criação e se o link é público ou privado.</li>
            <li>A senha dos links privados, protegida do mesmo jeito que a senha da sua conta.</li>
            <li>Quantas vezes cada link foi aberto.</li>
          </ul>
        </Section>

        <Section title="Quem abre os seus links">
          <p>
            Não guardamos nada sobre quem abre um link: nem quem é, nem de onde veio, nem qual aparelho usou. Cada acesso só
            soma um na contagem. Para barrar abusos, como alguém tentando adivinhar a senha de um link, olhamos o endereço de
            internet de quem acessa por alguns minutos, sem gravar.
          </p>
        </Section>

        <Section title="Cookies">
          <p>
            Usamos só os cookies necessários para o site funcionar: manter você conectado, concluir o cadastro e o login, e
            lembrar que você já digitou a senha de um link privado. Não usamos cookies de propaganda nem de rastreamento.
          </p>
        </Section>

        <Section title="Com quem compartilhamos">
          <p>
            Não vendemos seus dados e não os usamos para propaganda. Para o shortbit funcionar, contamos com empresas que
            hospedam o site, guardam os dados e enviam nossos emails, e com o Google, se você escolher entrar com ele. Algumas
            dessas empresas ficam fora do Brasil. Elas só usam os dados para manter o shortbit no ar.
          </p>
        </Section>

        <Section title="Por quanto tempo">
          <p>
            Seus dados e seus links ficam guardados enquanto a sua conta existir. Os códigos que enviamos por email valem por
            poucos minutos, e os registros internos de cliques são apagados em 7 dias.
          </p>
        </Section>

        <Section title="Seus direitos">
          <p>
            Você pode pedir uma cópia dos seus dados, corrigi-los ou excluir sua conta e seus links. Nome e senha você mesmo
            muda em Minha conta. Para o resto, escreva para{' '}
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
