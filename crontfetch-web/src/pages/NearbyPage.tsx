import { useCallback, useEffect, useState } from 'react'
import FilterBar from '../components/FilterBar'
import GoOnlineToggle from '../components/GoOnlineToggle'
import NearbyMap from '../components/NearbyMap'
import { fetchNearby, setCompanyLocation } from '../services/nearbyService'
import type { Company, NearbyCompany } from '../services/nearbyService'
import { fetchCompanies } from '../store/companies/companiesThunks'
import { useAppDispatch, useAppSelector } from '../store/hooks'

const RADIUS_KM = 10
// Used when the browser can't or won't share a location (central Bangalore).
const FALLBACK_POSITION: [number, number] = [12.9716, 77.5946]

function NearbyPage() {
  const dispatch = useAppDispatch()
  const token = useAppSelector((s) => s.auth.token)
  const companies = useAppSelector((s) => s.companies.companies) as Company[]

  const [position, setPosition] = useState<[number, number] | null>(null)
  const [locationNote, setLocationNote] = useState<string | null>(null)
  const [nearby, setNearby] = useState<NearbyCompany[]>([])
  const [error, setError] = useState<string | null>(null)
  const [pickingFor, setPickingFor] = useState<Company | null>(null)

  const locate = useCallback(() => {
    if (!navigator.geolocation) {
      setLocationNote('This browser has no location support. Showing central Bangalore.')
      setPosition(FALLBACK_POSITION)
      return
    }
    setLocationNote('Finding your location…')
    navigator.geolocation.getCurrentPosition(
      (p) => {
        setLocationNote(null)
        setPosition([p.coords.latitude, p.coords.longitude])
      },
      () => {
        setLocationNote('Location blocked or unavailable. Showing central Bangalore.')
        setPosition(FALLBACK_POSITION)
      },
      { enableHighAccuracy: true, timeout: 10000 },
    )
  }, [])

  const loadNearby = useCallback(async () => {
    if (!position) return
    try {
      setNearby(await fetchNearby(token, position[0], position[1], RADIUS_KM))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load nearby companies')
    }
  }, [position, token])

  useEffect(() => {
    locate()
    dispatch(fetchCompanies())
  }, [locate, dispatch])

  useEffect(() => {
    loadNearby()
  }, [loadNearby])

  const saveLocation = async (company: Company, body: { office_address?: string; lat?: number; lng?: number }) => {
    try {
      await setCompanyLocation(token, company.id, body)
      setError(null)
      dispatch(fetchCompanies())
      loadNearby()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not save location')
    }
  }

  const onPick = (lat: number, lng: number) => {
    if (!pickingFor) return
    const company = pickingFor
    setPickingFor(null)
    saveLocation(company, { office_address: company.office_address, lat, lng })
  }

  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>Jobs around me</h1>
          <p>Your tracked companies within {RADIUS_KM} km, with a Go online switch for match alerts.</p>
        </div>

        <GoOnlineToggle />
        <FilterBar />

        <div className="card nearby">
          <div className="nearby__bar">
            <span className="form-hint">
              {pickingFor
                ? `Click the map to place ${pickingFor.name}'s office.`
                : (locationNote ?? `${nearby.length} ${nearby.length === 1 ? 'company' : 'companies'} within ${RADIUS_KM} km`)}
            </span>
            {pickingFor ? (
              <button type="button" className="btn btn--secondary btn--sm" onClick={() => setPickingFor(null)}>
                Cancel
              </button>
            ) : (
              <button type="button" className="btn btn--secondary btn--sm" onClick={locate}>
                Refresh location
              </button>
            )}
          </div>

          {error && (
            <p className="alert alert--error" role="alert">
              {error}
            </p>
          )}

          {position && (
            <NearbyMap
              position={position}
              radiusKm={RADIUS_KM}
              companies={nearby}
              onPick={pickingFor ? onPick : undefined}
            />
          )}

          {nearby.length > 0 && (
            <ul className="nearby__list">
              {nearby.map((c) => (
                <li key={c.id} className="nearby__item">
                  <div>
                    <strong>{c.name}</strong>
                    <span className="form-hint">{c.office_address || 'Pinned on map'}</span>
                  </div>
                  <span className="nearby__distance">{c.distance_km.toFixed(1)} km</span>
                  <a href={c.career_url} target="_blank" rel="noreferrer" className="btn btn--primary btn--sm">
                    Career page ↗
                  </a>
                </li>
              ))}
            </ul>
          )}
        </div>

        {companies.length > 0 && (
          <div className="card nearby">
            <h2>Office locations</h2>
            <p className="form-hint">
              A company shows on the map once it has an office location. Type an address to look it up, or pick
              it on the map.
            </p>
            <ul className="nearby__list">
              {companies.map((c) => (
                <OfficeLocationRow
                  key={c.id}
                  company={c}
                  onSave={(body) => saveLocation(c, body)}
                  onPick={() => setPickingFor(c)}
                />
              ))}
            </ul>
          </div>
        )}
      </div>
    </section>
  )
}

interface OfficeLocationRowProps {
  company: Company
  onSave: (body: { office_address: string }) => Promise<void>
  onPick: () => void
}

function OfficeLocationRow({ company, onSave, onPick }: OfficeLocationRowProps) {
  const [address, setAddress] = useState(company.office_address)
  const [saving, setSaving] = useState(false)

  const lookUp = async () => {
    setSaving(true)
    await onSave({ office_address: address.trim() })
    setSaving(false)
  }

  return (
    <li className="nearby__item">
      <div>
        <strong>{company.name}</strong>
        <span className="form-hint">
          {company.lat != null ? `📍 ${company.lat.toFixed(4)}, ${company.lng!.toFixed(4)}` : 'No location yet'}
        </span>
      </div>
      <input
        className="input"
        placeholder="Office address, e.g. Koramangala, Bengaluru"
        value={address}
        onChange={(e) => setAddress(e.target.value)}
      />
      <div className="nearby__actions">
        <button
          type="button"
          className="btn btn--primary btn--sm"
          disabled={saving || address.trim() === ''}
          onClick={lookUp}
        >
          {saving ? 'Finding…' : 'Find address'}
        </button>
        <button type="button" className="btn btn--secondary btn--sm" onClick={onPick}>
          Pick on map
        </button>
      </div>
    </li>
  )
}

export default NearbyPage
