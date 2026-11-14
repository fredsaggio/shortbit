import { useEffect, useState } from 'react'
import { createChecker, type StrengthChecker, type StrengthScore } from './passwordStrengthCore.ts'

export type { StrengthScore } from './passwordStrengthCore.ts'

// Password strength from zxcvbn, which knows common passwords, names, words,
// keyboard runs and l33t, so "Senha@123" is weak however many symbols it has.
// OWASP recommends a meter like this over composition rules.
//
// The scoring runs in a Web Worker: on the main thread a long password froze
// typing for hundreds of milliseconds per key. The worker, and the few hundred
// kilobytes of dictionaries, load on first use, only when someone picks a
// password-protected link. Without Worker support (old browsers, tests) the
// same checker runs on the main thread.

const settleDelayMs = 150

let worker: Worker | undefined
let workerBroken = false
let nextRequestId = 0
const waiting = new Map<number, (score: StrengthScore | null) => void>()
let fallbackChecker: Promise<StrengthChecker> | undefined

function scoreOnMainThread(password: string): Promise<StrengthScore | null> {
  fallbackChecker ??= createChecker()
  return fallbackChecker.then((check) => check(password))
}

function startWorker(): Worker {
  const started = new Worker(new URL('./passwordStrength.worker.ts', import.meta.url), { type: 'module' })
  started.addEventListener('message', (event: MessageEvent<{ id: number; score: StrengthScore | null }>) => {
    waiting.get(event.data.id)?.(event.data.score)
    waiting.delete(event.data.id)
  })
  started.addEventListener('error', () => {
    // If the worker cannot load, score on the main thread from now on.
    workerBroken = true
    started.terminate()
    worker = undefined
    for (const resolve of waiting.values()) {
      resolve(null)
    }
    waiting.clear()
  })
  return started
}

function score(password: string): Promise<StrengthScore | null> {
  if (typeof Worker === 'undefined' || workerBroken) {
    return scoreOnMainThread(password)
  }

  worker ??= startWorker()
  const id = ++nextRequestId
  const request = new Promise<StrengthScore | null>((resolve) => waiting.set(id, resolve))
  worker.postMessage({ id, password })
  return request.then((result) => (result === null && workerBroken ? scoreOnMainThread(password) : result))
}

/**
 * The zxcvbn score of `password`, or null while it is empty or not scored yet.
 * It scores once typing settles, and only the latest password: older results
 * are dropped. Until the new score arrives, the last one stays on screen.
 */
export function usePasswordStrength(password: string): StrengthScore | null {
  const [result, setResult] = useState<{ password: string; score: StrengthScore } | null>(null)

  useEffect(() => {
    if (!password) {
      return
    }

    let current = true
    const timer = window.setTimeout(() => {
      score(password).then(
        (value) => {
          if (current && value !== null) {
            setResult({ password, score: value })
          }
        },
        () => {
          // Without the meter the form still works; the API checks the length.
        },
      )
    }, settleDelayMs)

    return () => {
      current = false
      window.clearTimeout(timer)
    }
  }, [password])

  return password && result ? result.score : null
}
