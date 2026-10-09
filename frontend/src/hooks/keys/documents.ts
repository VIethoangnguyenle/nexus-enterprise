export const documentKeys = {
  list: (wsId: string) => ['documents', wsId] as const,
  detail: (id: string) => ['document', id] as const,
}
