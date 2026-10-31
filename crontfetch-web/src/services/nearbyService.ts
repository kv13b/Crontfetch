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
