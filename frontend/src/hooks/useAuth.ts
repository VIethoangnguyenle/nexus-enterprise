import { useMutation } from '@tanstack/react-query'
import { authApi, type LoginPayload, type RegisterPayload, type OTPRequestPayload, type OTPVerifyPayload } from '../api/auth'
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

export function useRequestOTP() {
  return useMutation({
    mutationFn: (data: OTPRequestPayload) => authApi.requestOTP(data),
  })
}

export function useVerifyOTP() {
  const login = useAuthStore((s) => s.login)
  return useMutation({
    mutationFn: (data: OTPVerifyPayload) => authApi.verifyOTP(data),
    onSuccess: (res) => login(res.token, res.user),
  })
}
