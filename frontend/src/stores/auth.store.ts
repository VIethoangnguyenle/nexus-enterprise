import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  id: string
  username: string
  ngac_node_id?: string
}

interface AuthState {
  token: string | null
  user: User | null
  login: (token: string, user: User) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,

      login: (token, user) => set({ token, user }),

      logout: () => {
        set({ token: null, user: null })
        window.location.href = '/login'
      },

      isAuthenticated: () => !!get().token,
    }),
    { name: 'ngac-auth' },
  ),
)
