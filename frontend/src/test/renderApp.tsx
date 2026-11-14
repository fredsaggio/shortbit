import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { App } from '../App.tsx'
import { CurrentPath } from './CurrentPath.tsx'

export function renderApp(path: string) {
  const userActions = userEvent.setup()
  const view = render(
    <MemoryRouter initialEntries={[path]}>
      <App />
      <CurrentPath />
    </MemoryRouter>,
  )
  return { ...view, user: userActions }
}
