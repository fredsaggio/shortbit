import { describe, expect, it } from 'vitest'
import { checkCode, checkConfirmation, checkEmail, checkFullName, checkNewPassword, checkURL } from './validation.ts'

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
})
