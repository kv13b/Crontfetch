import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import TagInput from '../components/TagInput'
import { fetchPreferences, savePreferences } from '../services/profileService'
import type { Preferences } from '../services/profileService'
import { useAppSelector } from '../store/hooks'

const toNumber = (s: string): number | null => (s.trim() === '' ? null : Number(s))

function validExperience(n: number | null): boolean {
  return n === null || (Number.isInteger(n) && n >= 0)
}

function ProfilePage() {
  const token = useAppSelector((s) => s.auth.token)

  const [locations, setLocations] = useState<string[]>([])
  const [roles, setRoles] = useState<string[]>([])
  const [interests, setInterests] = useState<string[]>([])
  const [minExp, setMinExp] = useState('')
  const [maxExp, setMaxExp] = useState('')

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  const fillForm = (p: Preferences) => {
    setLocations(p.locations)
    setRoles(p.roles)
    setInterests(p.interests)
    setMinExp(p.min_experience_years?.toString() ?? '')
    setMaxExp(p.max_experience_years?.toString() ?? '')
  }

  useEffect(() => {
    let cancelled = false
    fetchPreferences(token)
      .then((p) => {
        if (!cancelled) fillForm(p)
      })
      .catch((e: Error) => {
        if (!cancelled) setError(e.message)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [token])

  // The "saved" message fades out on its own.
  useEffect(() => {
    if (!saved) return
    const timer = setTimeout(() => setSaved(false), 3000)
    return () => clearTimeout(timer)
  }, [saved])

  // Any edit hides the old "Saved" message.
  const edited =
    <T,>(set: (v: T) => void) =>
    (v: T) => {
      set(v)
      setSaved(false)
    }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSaved(false)

    const min = toNumber(minExp)
    const max = toNumber(maxExp)
    if (!validExperience(min) || !validExperience(max)) {
      setError('Experience must be a whole number of years, 0 or more.')
      return
    }
    if (min !== null && max !== null && min > max) {
      setError('Minimum experience cannot be more than maximum.')
      return
    }

    setSaving(true)
    setError(null)
    try {
      // Show what the server stored (trimmed, blanks dropped).
      fillForm(
        await savePreferences(token, {
          locations,
          roles,
          interests,
          min_experience_years: min,
          max_experience_years: max,
        }),
      )
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not save preferences')
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>Profile settings</h1>
          <p>Tell us what you're looking for. Search jobs and Jobs around me start from these.</p>
        </div>

        <div className="card">
          {loading ? (
            <p>Loading your preferences…</p>
          ) : (
            <form className="form" onSubmit={onSubmit} noValidate>
              <div className="form-field">
                <label className="form-label" htmlFor="pref-locations">
                  Location
                </label>
                <TagInput
                  id="pref-locations"
                  values={locations}
                  onChange={edited(setLocations)}
                  placeholder="e.g. Bangalore, Delhi, Remote"
                />
                <span className="form-hint">Where you want to work. Press Enter to add.</span>
              </div>

              <div className="form-field">
                <label className="form-label" htmlFor="pref-roles">
                  Role
                </label>
                <TagInput
                  id="pref-roles"
                  values={roles}
                  onChange={edited(setRoles)}
                  placeholder="e.g. Full stack developer, HR"
                />
                <span className="form-hint">Job titles you are looking for.</span>
              </div>

              <div className="form-field">
                <label className="form-label" htmlFor="pref-interests">
                  Area of interest
                </label>
                <TagInput
                  id="pref-interests"
                  values={interests}
                  onChange={edited(setInterests)}
                  placeholder="e.g. React, Java, Finance"
                />
                <span className="form-hint">Stacks or fields you care about.</span>
              </div>

              <div className="form-row">
                <div className="form-field">
                  <label className="form-label" htmlFor="pref-min-exp">
                    Min experience (years)
                  </label>
                  <input
                    id="pref-min-exp"
                    className="input"
                    type="number"
                    inputMode="numeric"
                    min={0}
                    step={1}
                    placeholder="e.g. 2"
                    value={minExp}
                    onChange={(e) => edited(setMinExp)(e.target.value)}
                  />
                </div>
                <div className="form-field">
                  <label className="form-label" htmlFor="pref-max-exp">
                    Max experience (years)
                  </label>
                  <input
                    id="pref-max-exp"
                    className="input"
                    type="number"
                    inputMode="numeric"
                    min={0}
                    step={1}
                    placeholder="e.g. 5"
                    value={maxExp}
                    onChange={(e) => edited(setMaxExp)(e.target.value)}
                  />
                </div>
              </div>

              {error && (
                <p className="alert alert--error" role="alert">
                  {error}
                </p>
              )}
              {saved && (
                <p className="alert alert--success" role="status">
                  Preferences saved.
                </p>
              )}

              <div>
                <button type="submit" className="btn btn--primary" disabled={saving}>
                  {saving ? 'Saving…' : 'Save preferences'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </section>
  )
}

export default ProfilePage
