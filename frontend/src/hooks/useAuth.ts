import { useMutation } from '@tanstack/react-query'
import { authApi, type LoginPayload, type RegisterPayload } from '../api/auth'
import { useAuthStore } from '../stores/auth.store'

export function useLogin() {
  const login = useAuthStore((s) => s.login)
  return useMutation({
    mutationFn: (data: LoginPayload) => authApi.login(data),
    onSuccess: (res) => login(res.token, res.user),
  })
}

export function useRegister() {
  const login = useAuthStore((s) => s.login)
  return useMutation({
    mutationFn: (data: RegisterPayload) => authApi.register(data),
    onSuccess: (res) => login(res.token, res.user),
  })
}
