export const workspaceKeys = {
  all: () => ['workspaces'] as const,
  details: (wsId: string) => ['workspaces', wsId, 'details'] as const,
}
