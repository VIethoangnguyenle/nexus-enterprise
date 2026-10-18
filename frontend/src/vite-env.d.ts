/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** `YYYY.MM.DD · <short sha>` of the build; unset in local dev. */
  readonly VITE_APP_VERSION?: string
}
