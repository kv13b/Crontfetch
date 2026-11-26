import { request } from './api'

// API contract for the Telegram connect flow (backend not built yet):
//   GET    /me/telegram       -> { connected: boolean }
//   POST   /me/telegram/link  -> { url: "https://t.me/<bot>?start=<code>", expires_at: ISO time }
//   DELETE /me/telegram       -> 204, no body

export interface TelegramStatus {
  connected: boolean
}

export interface TelegramLink {
  url: string
  expires_at: string
}

export const fetchTelegramStatus = (token: string | null) =>
  request<TelegramStatus>('/me/telegram', { token })

// Each call makes a fresh one-time code, so an expired link is fixed by asking again.
export const createTelegramLink = (token: string | null) =>
  request<TelegramLink>('/me/telegram/link', { method: 'POST', token })

export const disconnectTelegram = (token: string | null) =>
  request<null>('/me/telegram', { method: 'DELETE', token })
