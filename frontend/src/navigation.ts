export type ReturnState = { from?: string } | null

// Only paths inside this app are accepted as a return destination, so a
// crafted location state cannot send the user to another site.
export function returnPath(state: unknown): string {
  const from = (state as ReturnState)?.from
  return typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/painel'
}
