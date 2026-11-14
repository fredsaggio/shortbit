import { describe, expect, it } from 'vitest'
import { checkCode, checkConfirmation, checkEmail, checkFullName, checkLinkPassword, checkNewPassword, checkURL, passwordRules } from './validation.ts'

describe('validation', () => {
  it('accepts valid values', () => {
    expect(checkEmail(' ana@example.com ')).toBeUndefined()
    expect(checkNewPassword('senha123')).toBeUndefined()
    expect(checkConfirmation('senha123', 'senha123')).toBeUndefined()
    expect(checkFullName('Ana Souza')).toBeUndefined()
    expect(checkURL('https://example.com/a?b=c')).toBeUndefined()
    expect(checkCode('123456', 6)).toBeUndefined()
  })

  it('counts characters, not bytes, in passwords', () => {
    expect(checkNewPassword('çãõéíúâê')).toBeUndefined()
    expect(checkNewPassword('ção1234')).toBe('A senha precisa ter pelo menos 8 caracteres.')
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
    // 512 two-byte characters: long enough in characters, 1024 bytes exactly.
    expect(checkNewPassword('é'.repeat(512))).toBeUndefined()
    expect(checkNewPassword('é'.repeat(513))).toBe('A senha está longa demais. Use uma mais curta.')
    expect(checkLinkPassword('a'.repeat(1025))).toBe('A senha do link está longa demais. Use uma mais curta.')
  })

  it('lists exactly the rules the API enforces', () => {
    const state = (rules: ReturnType<typeof passwordRules>) => rules.map((rule) => [rule.label, rule.met])

    expect(state(passwordRules({ password: 'curta' }))).toEqual([['Pelo menos 8 caracteres', false]])
    expect(state(passwordRules({ password: 'uma senha bem comprida', confirmation: 'uma senha bem comprida', current: 'antiga123' }))).toEqual([
      ['Pelo menos 8 caracteres', true],
      ['Diferente da senha atual', true],
      ['As duas senhas são iguais', true],
    ])
    expect(passwordRules({ password: 'antiga123', current: 'antiga123' })[1]).toEqual({ label: 'Diferente da senha atual', met: false })
    expect(passwordRules({ password: '', confirmation: '' })[1].met).toBe(false)
    // Counts characters, not UTF-16 units, like the API.
    expect(passwordRules({ password: '😀'.repeat(8) })[0].met).toBe(true)
    expect(passwordRules({ password: 'a'.repeat(1025) }).map((rule) => rule.label)).toContain('Não muito longa (o limite foi ultrapassado)')
  })
})
