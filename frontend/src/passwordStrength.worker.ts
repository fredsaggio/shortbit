// Scores passwords off the main thread, so typing never stalls on zxcvbn.
import { createChecker } from './passwordStrengthCore.ts'

type Request = { id: number; password: string }

const checker = createChecker()
const scope = self as unknown as { postMessage(message: unknown): void }

self.addEventListener('message', (event: MessageEvent<Request>) => {
  const { id, password } = event.data
  checker
    .then((check) => check(password))
    .then(
      (score) => scope.postMessage({ id, score }),
      () => scope.postMessage({ id, score: null }),
    )
})
