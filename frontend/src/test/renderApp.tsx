import { render } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, useLocation } from 'react-router'
import { App } from '../App.tsx'

function CurrentPath() {
  const location = useLocation()
  return <output data-testid="current-path">{location.pathname + location.search}</output>
}

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
