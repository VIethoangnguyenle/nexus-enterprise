export const adminKeys = {
  departments: (wsId: string) => ['admin', 'departments', wsId] as const,
  members: (wsId: string) => ['admin', 'members', wsId] as const,
  roles: (wsId: string) => ['admin', 'roles', wsId] as const,
}
