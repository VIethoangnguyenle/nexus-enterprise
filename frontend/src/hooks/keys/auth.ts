export const authKeys = {
  providers: () => ['auth', 'providers'] as const,
  /** The signed-in person: name, address, whether it is verified, whether a profile is owed. */
  me: () => ['auth', 'me'] as const,
  /** Their profile as Settings shows it, with their place in one workspace. */
  profile: (workspaceId: string) => ['auth', 'profile', workspaceId] as const,
  /** Every profile read: what a save invalidates. */
  profiles: () => ['auth', 'profile'] as const,
  /** The workspaces they belong to, with role and headcount. */
  workspaces: () => ['auth', 'workspaces'] as const,
  /** Offers addressed to them. */
  invitations: () => ['auth', 'invitations'] as const,
}
