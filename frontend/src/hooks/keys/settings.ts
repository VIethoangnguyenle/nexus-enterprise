/** A person's settings of one service in one scope (`workspace` + a workspace id). */
export const settingsKeys = {
  all: () => ['settings'] as const,
  scope: (service: string, scopeId: string) => ['settings', service, scopeId] as const,
}
