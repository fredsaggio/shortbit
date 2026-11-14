import { useLocation } from 'react-router'

export function CurrentPath() {
  const location = useLocation()
  return <output data-testid="current-path">{location.pathname + location.search}</output>
}
