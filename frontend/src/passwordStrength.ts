import { useEffect, useState } from 'react'

// Password strength from zxcvbn, which knows common passwords, names, words,
// keyboard runs and l33t, so "Senha@123" is weak however many symbols it has.
// OWASP recommends a meter like this over composition rules.
//
// Its dictionaries are a few hundred kilobytes, so they load on first use,
// only when someone picks a password-protected link.

/** 0 (very weak) to 4 (very strong), as zxcvbn scores. */
export type StrengthScore = 0 | 1 | 2 | 3 | 4

let checker: Promise<(password: string) => Promise<StrengthScore>> | undefined

function loadChecker() {
  checker ??= Promise.all([import('@zxcvbn-ts/core'), import('@zxcvbn-ts/language-common'), import('@zxcvbn-ts/language-pt-br')]).then(
    ([core, common, portuguese]) => {
      const zxcvbn = new core.ZxcvbnFactory({
        dictionary: { ...common.dictionary, ...portuguese.dictionary },
        graphs: common.adjacencyGraphs,
        translations: portuguese.translations,
      })
      return async (password: string) => (await zxcvbn.check(password)).score as StrengthScore
    },
  )
  return checker
}

/**
 * The zxcvbn score of `password`, or null while it is empty or the checker is
 * still loading. While typing, it keeps the last score until the new one is in.
 */
export function usePasswordStrength(password: string): StrengthScore | null {
  const [result, setResult] = useState<{ password: string; score: StrengthScore } | null>(null)

  useEffect(() => {
    if (!password) {
      return
    }

    let current = true
    loadChecker()
      .then((check) => check(password))
      .then(
        (score) => {
          if (current) {
            setResult({ password, score })
          }
        },
        () => {
          // Without the meter the form still works; the API checks the length.
        },
      )
    return () => {
      current = false
    }
  }, [password])

  return password && result ? result.score : null
}
