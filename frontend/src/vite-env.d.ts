/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SHORT_LINK_BASE_URL?: string
  readonly VITE_API_PROXY_TARGET?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
