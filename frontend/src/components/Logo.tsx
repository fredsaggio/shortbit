export function Logo() {
  return (
    <span className="logo">
      <svg className="logo__mark" viewBox="0 0 32 32" aria-hidden="true">
        <rect width="32" height="32" rx="8" fill="currentColor" />
        <path d="M13.5 18.5l5-5" stroke="var(--cream)" strokeWidth="2.6" strokeLinecap="round" />
        <path
          d="M15 11.2l1.6-1.6a4.4 4.4 0 0 1 6.2 6.2L21.2 17.4"
          fill="none"
          stroke="var(--caramel)"
          strokeWidth="2.6"
          strokeLinecap="round"
        />
        <path
          d="M17 20.8l-1.6 1.6a4.4 4.4 0 0 1-6.2-6.2l1.6-1.6"
          fill="none"
          stroke="var(--cream)"
          strokeWidth="2.6"
          strokeLinecap="round"
        />
      </svg>
      <span className="logo__text">
        Short<span className="logo__accent">Bit</span>
      </span>
    </span>
  )
}
