import { describe, expect, it } from 'vitest'
import { checkCode, checkConfirmation, checkEmail, checkFullName, checkLinkPassword, checkNewPassword, checkURL, passwordRules } from './validation.ts'

describe('validation', () => {
  it('accepts valid values', () => {
    expect(checkEmail(' ana@example.com ')).toBeUndefined()
    expect(checkNewPassword('Senha12!')).toBeUndefined()
    expect(checkConfirmation('senha123', 'senha123')).toBeUndefined()
    expect(checkFullName('Ana Souza')).toBeUndefined()
    expect(checkURL('https://example.com/a?b=c')).toBeUndefined()
    expect(checkCode('123456', 6)).toBeUndefined()
  })

  it('counts characters, not bytes, in passwords', () => {
    expect(checkNewPassword('Çãõéíú1!')).toBeUndefined()
    expect(checkNewPassword('Çã1!abc')).toBe('A senha precisa ter pelo menos 8 caracteres.')
  })

  it('applies the same composition rules as the API to account passwords', () => {
    expect(checkNewPassword('senha-segura1')).toBe('A senha precisa ter pelo menos uma letra maiúscula.')
    expect(checkNewPassword('Senha-segura')).toBe('A senha precisa ter pelo menos um número.')
    expect(checkNewPassword('SenhaSegura1')).toBe('A senha precisa ter pelo menos um caractere especial, como ! @ # ou -.')
    // A space is not special; an accented capital and an emoji count.
    expect(checkNewPassword('Senha Segura1')).toBe('A senha precisa ter pelo menos um caractere especial, como ! @ # ou -.')
    expect(checkNewPassword('Érica#2026')).toBeUndefined()
    expect(checkNewPassword('Senha1234😀')).toBeUndefined()
    // Link passwords keep only the length rule.
    expect(checkLinkPassword('senhadolink')).toBeUndefined()
  })

  it('rejects URLs that are not http or https', () => {
    expect(checkURL('javascript:alert(1)')).toBe('Digite uma URL completa, começando com http:// ou https://.')
    expect(checkURL('ftp://example.com')).toBe('Digite uma URL completa, começando com http:// ou https://.')
    expect(checkURL('   ')).toBe('Cole a URL que você quer encurtar.')
  })

  it('limits the full name', () => {
    expect(checkFullName('   ')).toBe('Informe seu nome completo.')
    expect(checkFullName('a'.repeat(101))).toBe('O nome pode ter até 100 caracteres.')
  })

  it('checks code length and digits', () => {
    expect(checkCode('12345', 6)).toBe('O código tem 6 dígitos.')
    expect(checkCode('', 8)).toBe('Digite o código que chegou no seu email.')
  })

  it('rejects passwords over the API limit of 1024 bytes', () => {
    // É and é take two bytes, 1 and ! one each: 1024 bytes exactly.
    expect(checkNewPassword('É' + 'é'.repeat(510) + '1!')).toBeUndefined()
    expect(checkNewPassword('É' + 'é'.repeat(511) + '1!')).toBe('A senha está longa demais. Use uma mais curta.')
    expect(checkLinkPassword('a'.repeat(1025))).toBe('A senha do link está longa demais. Use uma mais curta.')
  })

  it('lists exactly the rules the API enforces', () => {
    const state = (rules: ReturnType<typeof passwordRules>) => rules.map((rule) => [rule.label, rule.met])

    expect(state(passwordRules({ password: 'curta' }))).toEqual([
      ['Pelo menos 8 caracteres', false],
      ['Uma letra maiúscula', false],
      ['Um número', false],
      ['Um caractere especial (! @ # - …)', false],
    ])
    expect(state(passwordRules({ password: 'Senha-nova-123', current: 'Senha-antiga1' }))).toEqual([
      ['Pelo menos 8 caracteres', true],
      ['Uma letra maiúscula', true],
      ['Um número', true],
      ['Um caractere especial (! @ # - …)', true],
      ['Diferente da senha atual', true],
    ])
    expect(passwordRules({ password: 'Antiga-123', current: 'Antiga-123' })[4]).toEqual({ label: 'Diferente da senha atual', met: false })
    expect(passwordRules({ password: '' }).every((rule) => !rule.met)).toBe(true)
    // Counts characters, not UTF-16 units, like the API.
    expect(passwordRules({ password: '😀'.repeat(8) })[0].met).toBe(true)
    expect(passwordRules({ password: 'a'.repeat(1025) }).map((rule) => rule.label)).toContain('Não muito longa (o limite foi ultrapassado)')
  })
})
