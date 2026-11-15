import { request } from './api'

// What the user is looking for, saved on their account.
export interface Preferences {
  locations: string[]
  roles: string[]
  interests: string[]
  min_experience_years: number | null
  max_experience_years: number | null
}

export const fetchPreferences = (token: string | null) =>
  request<Preferences>('/me/preferences', { token })

// Replaces every field, so always send the whole object.
export const savePreferences = (token: string | null, body: Preferences) =>
  request<Preferences>('/me/preferences', { method: 'PUT', body, token })
