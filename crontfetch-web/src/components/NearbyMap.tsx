import { useEffect } from 'react'
import { Circle, CircleMarker, MapContainer, Popup, TileLayer, useMap, useMapEvents } from 'react-leaflet'
import 'leaflet/dist/leaflet.css'
import type { NearbyCompany, NearbyPlace } from '../services/nearbyService'

interface NearbyMapProps {
  position: [number, number]
  radiusKm: number
  companies: NearbyCompany[]
  // Offices from OpenStreetMap that the user doesn't track yet.
  places: NearbyPlace[]
  onTrack: (place: NearbyPlace) => void
  // When set, the next click on the map reports its coordinates.
  onPick?: (lat: number, lng: number) => void
}

// Circle markers instead of Leaflet's default pin images, which need extra
// bundler setup to load.
function NearbyMap({ position, radiusKm, companies, places, onTrack, onPick }: NearbyMapProps) {
  return (
    <MapContainer center={position} zoom={13} className={`nearby-map${onPick ? ' nearby-map--picking' : ''}`}>
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <Recenter position={position} />
      <ClickToPick onPick={onPick} />

      <Circle center={position} radius={radiusKm * 1000} pathOptions={{ color: '#5b4cf0', weight: 1, fillOpacity: 0.05 }} />

      {/* Untracked offices first, so tracked pins and "you" draw on top. */}
      {places.map((p) => (
        <CircleMarker
          key={p.id}
          center={[p.lat, p.lng]}
          radius={5}
          pathOptions={{ color: '#fff', weight: 1, fillColor: '#8a8aa0', fillOpacity: 0.9 }}
        >
          <Popup>
            <strong>{p.name}</strong>
            <br />
            {p.distance_km.toFixed(1)} km away
            {p.address && (
              <>
                <br />
                <span className="nearby-popup__muted">{p.address}</span>
              </>
            )}
            {p.website && (
              <>
                <br />
                <a href={p.website} target="_blank" rel="noreferrer">
                  Website ↗
                </a>
              </>
            )}
            <br />
            <button type="button" className="btn btn--primary btn--sm nearby-popup__btn" onClick={() => onTrack(p)}>
              Track this company
            </button>
          </Popup>
        </CircleMarker>
      ))}

      {companies.map((c) => (
        <CircleMarker
          key={c.id}
          center={[c.lat!, c.lng!]}
          radius={8}
          pathOptions={{ color: '#fff', weight: 2, fillColor: '#15a34a', fillOpacity: 1 }}
        >
          <Popup>
            <strong>{c.name}</strong> (tracked)
            <br />
            {c.distance_km.toFixed(1)} km away
            <br />
            <a href={c.career_url} target="_blank" rel="noreferrer">
              Career page ↗
            </a>
          </Popup>
        </CircleMarker>
      ))}

      <CircleMarker center={position} radius={8} pathOptions={{ color: '#fff', weight: 2, fillColor: '#5b4cf0', fillOpacity: 1 }}>
        <Popup>You are here</Popup>
      </CircleMarker>
    </MapContainer>
  )
}

// MapContainer only reads `center` once, so follow position changes here.
function Recenter({ position }: { position: [number, number] }) {
  const map = useMap()
  useEffect(() => {
    map.setView(position)
  }, [map, position])
  return null
}

function ClickToPick({ onPick }: { onPick?: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(e) {
      onPick?.(e.latlng.lat, e.latlng.lng)
    },
  })
  return null
}

export default NearbyMap
