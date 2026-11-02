import { request } from './api'

export interface Company {
  id: string
  name: string
  career_url: string
  office_address: string
  lat: number | null
  lng: number | null
}

export interface NearbyCompany extends Company {
  distance_km: number
}

export const fetchNearby = (token: string | null, lat: number, lng: number, radiusKm = 10) =>
  request<NearbyCompany[]>(`/nearby?lat=${lat}&lng=${lng}&radius_km=${radiusKm}`, { token })

// Send lat/lng to place a pin directly, or only office_address to have the
// server look it up.
export const setCompanyLocation = (
  token: string | null,
  id: string,
  body: { office_address?: string; lat?: number; lng?: number },
) => request<Company>(`/companies/${id}/location`, { method: 'PUT', body, token })

// An office from OpenStreetMap near the user, tracked or not.
export interface NearbyPlace {
  id: string
  name: string
  kind: string
  website: string
  address: string
  lat: number
  lng: number
  distance_km: number
  source: string
}

export const fetchNearbyPlaces = (token: string | null, lat: number, lng: number, radiusKm = 10) =>
  request<NearbyPlace[]>(`/nearby/places?lat=${lat}&lng=${lng}&radius_km=${radiusKm}`, { token })

// What Jobs around me hands to Add Jobs when the user tracks a nearby office.
export interface TrackPrefill {
  name: string
  career_url: string
  office_address: string
  lat: number
  lng: number
}

// A suggested career page for a company, for the user to check before saving.
export interface CareerPageSuggestion {
  career_url: string
  platform: string
  board: string
  // A job board CronFetch can read was found, not just a page.
  found: boolean
  // How it was found, in plain words.
  how: string
}

export const findCareerPage = (token: string | null, name: string, website: string) =>
  request<CareerPageSuggestion>(
    `/career-page?name=${encodeURIComponent(name)}&website=${encodeURIComponent(website)}`,
    { token },
  )
