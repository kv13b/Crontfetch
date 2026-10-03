import type { AuthResponse, LoginPayload, SignupPayload } from '../types/auth'
import { request } from './api'

export const signup = (payload: SignupPayload) =>
  request<AuthResponse>('/signup', { method: 'POST', body: payload })

export const login = (payload: LoginPayload) =>
  request<AuthResponse>('/login', { method: 'POST', body: payload })
