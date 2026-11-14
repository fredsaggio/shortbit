// Builds the zxcvbn checker. Shared by the Web Worker and by the main-thread
// fallback, and kept apart from React so the worker does not pull React in.

/** 0 (very weak) to 4 (very strong), as zxcvbn scores. */
export type StrengthScore = 0 | 1 | 2 | 3 | 4

// zxcvbn's cost grows quickly with length: a random 80-character password
// took about half a second to score. The first 64 characters are plenty to
// tell weak from strong; the API still accepts up to 1024 bytes.
export const maxAnalyzedLength = 64

export type StrengthChecker = (password: string) => Promise<StrengthScore>

export async function createChecker(): Promise<StrengthChecker> {
  const [core, common, portuguese] = await Promise.all([
    import('@zxcvbn-ts/core'),
    import('@zxcvbn-ts/language-common'),
    import('@zxcvbn-ts/language-pt-br'),
  ])
  const zxcvbn = new core.ZxcvbnFactory({
    dictionary: { ...common.dictionary, ...portuguese.dictionary },
    graphs: common.adjacencyGraphs,
    translations: portuguese.translations,
    maxLength: maxAnalyzedLength,
  })
  return async (password) => (await zxcvbn.check(password)).score as StrengthScore
}
