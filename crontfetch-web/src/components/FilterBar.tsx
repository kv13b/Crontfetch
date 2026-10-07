import { useEffect, useRef, useState } from 'react'
import { setFilter } from '../store/filter'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { emptyFilter, isFilterEmpty } from '../types/filter'
import type { JobFilter } from '../types/filter'
import TagInput from './TagInput'

type TagKey = 'locations' | 'roles' | 'interests'
type OpenKey = TagKey | 'experience' | null

const tagFields: { key: TagKey; label: string; placeholder: string; hint: string }[] = [
  {
    key: 'locations',
    label: 'Location',
    placeholder: 'e.g. Bangalore, Delhi, Remote',
    hint: 'Where you want to work. Press Enter to add.',
  },
  {
    key: 'roles',
    label: 'Role',
    placeholder: 'e.g. Full stack developer, HR',
    hint: 'Job titles you are looking for.',
  },
  {
    key: 'interests',
    label: 'Area of interest',
    placeholder: 'e.g. React, Java, Finance',
    hint: 'Stacks or fields you care about.',
  },
]

const toNumber = (s: string): number | null => (s.trim() === '' ? null : Number(s))

function tagSummary(label: string, values: string[]): string {
  if (values.length === 0) return label
  if (values.length === 1) return `${label}: ${values[0]}`
  return `${label}: ${values[0]} +${values.length - 1}`
}

function experienceSummary(years: number | null): string {
  if (years === null) return 'Experience'
  return `Experience: ${years} ${years === 1 ? 'yr' : 'yrs'}`
}

interface TagPopoverProps {
  field: (typeof tagFields)[number]
  initial: string[]
  onApply: (values: string[]) => void
}

function TagPopover({ field, initial, onApply }: TagPopoverProps) {
  const [draft, setDraft] = useState(initial)

  return (
    <div className="filter-popover" role="dialog" aria-label={field.label}>
      <label className="form-label" htmlFor={`filter-${field.key}`}>
        {field.label}
      </label>
      <TagInput
        id={`filter-${field.key}`}
        values={draft}
        onChange={setDraft}
        placeholder={field.placeholder}
      />
      <span className="form-hint">{field.hint}</span>
      <div className="filter-popover__actions">
        <button type="button" className="btn btn--ghost" onClick={() => onApply([])}>
          Clear
        </button>
        <button type="button" className="btn btn--primary" onClick={() => onApply(draft)}>
          Apply
        </button>
      </div>
    </div>
  )
}

interface ExperiencePopoverProps {
  years: number | null
  onApply: (years: number | null) => void
}

function ExperiencePopover({ years, onApply }: ExperiencePopoverProps) {
  const [text, setText] = useState(years?.toString() ?? '')
  const [error, setError] = useState<string | null>(null)

  const apply = () => {
    const next = toNumber(text)
    if (next !== null && (!Number.isFinite(next) || next < 0)) {
      setError('Enter your years of experience as a number, 0 or more.')
    } else {
      onApply(next)
    }
  }

  return (
    <div className="filter-popover" role="dialog" aria-label="Experience">
      <div className="form-field">
        <label className="form-label" htmlFor="filter-experience">
          Years of experience
        </label>
        <input
          id="filter-experience"
          className="input"
          type="number"
          inputMode="decimal"
          min={0}
          step="any"
          placeholder="e.g. 3"
          value={text}
          onChange={(e) => {
            setText(e.target.value)
            setError(null)
          }}
        />
        <span className="form-hint">Your own experience. Use 0 for a fresher.</span>
      </div>
      {error && (
        <p className="alert alert--error" role="alert">
          {error}
        </p>
      )}
      <div className="filter-popover__actions">
        <button type="button" className="btn btn--ghost" onClick={() => onApply(null)}>
          Clear
        </button>
        <button type="button" className="btn btn--primary" onClick={apply}>
          Apply
        </button>
      </div>
    </div>
  )
}

// LinkedIn-style filter row. Each pill opens a small popover; applied
// filters are shown on the pill. The filter is shared by every page that
// renders this bar.
function FilterBar() {
  const dispatch = useAppDispatch()
  const filter = useAppSelector((s) => s.filter.filter)
  const [openKey, setOpenKey] = useState<OpenKey>(null)
  const rootRef = useRef<HTMLDivElement>(null)

  // Close on a click outside the bar or on Escape.
  useEffect(() => {
    if (!openKey) return

    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpenKey(null)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpenKey(null)
    }

    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [openKey])

  const apply = (patch: Partial<JobFilter>) => {
    dispatch(setFilter({ ...filter, ...patch }))
    setOpenKey(null)
  }

  const toggle = (key: Exclude<OpenKey, null>) => setOpenKey(openKey === key ? null : key)

  const pill = (key: Exclude<OpenKey, null>, label: string, active: boolean) => (
    <button
      type="button"
      className={`filter-pill${active ? ' filter-pill--active' : ''}`}
      aria-haspopup="dialog"
      aria-expanded={openKey === key}
      onClick={() => toggle(key)}
    >
      <span className="filter-pill__label">{label}</span>
      <span className="filter-pill__chevron" aria-hidden="true" />
    </button>
  )

  return (
    <div className="filter-bar" ref={rootRef} role="group" aria-label="Job filters">
      {tagFields.map((field) => (
        <div className="filter-bar__item" key={field.key}>
          {pill(field.key, tagSummary(field.label, filter[field.key]), filter[field.key].length > 0)}
          {openKey === field.key && (
            <TagPopover
              field={field}
              initial={filter[field.key]}
              onApply={(values) => apply({ [field.key]: values })}
            />
          )}
        </div>
      ))}

      <div className="filter-bar__item">
        {pill(
          'experience',
          experienceSummary(filter.experienceYears),
          filter.experienceYears !== null,
        )}
        {openKey === 'experience' && (
          <ExperiencePopover
            years={filter.experienceYears}
            onApply={(years) => apply({ experienceYears: years })}
          />
        )}
      </div>

      {!isFilterEmpty(filter) && (
        <button type="button" className="btn btn--ghost" onClick={() => apply(emptyFilter)}>
          Clear all
        </button>
      )}
    </div>
  )
}

export default FilterBar
