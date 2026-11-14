import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { MemoryRouter } from 'react-router'
import { App } from '../App.tsx'
import { CurrentPath } from './CurrentPath.tsx'

export function renderApp(path: string) {
  const userActions = userEvent.setup()
  // StrictMode runs effects twice, as in development, so tests catch effects
  // that are not safe to repeat.
  const view = render(
    <StrictMode>
      <MemoryRouter initialEntries={[path]}>
        <App />
        <CurrentPath />
      </MemoryRouter>
    </StrictMode>,
  )
  return { ...view, user: userActions }
}
