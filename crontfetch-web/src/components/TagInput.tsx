import { useState } from 'react'
import type { KeyboardEvent } from 'react'

interface TagInputProps {
  id: string
  values: string[]
  onChange: (values: string[]) => void
  placeholder?: string
}

// Free-text list: type a value and press Enter or comma to add it as a chip.
function TagInput({ id, values, onChange, placeholder }: TagInputProps) {
  const [draft, setDraft] = useState('')

  const add = (raw: string) => {
    const value = raw.trim()
    setDraft('')
    if (!value) return
    if (values.some((v) => v.toLowerCase() === value.toLowerCase())) return
    onChange([...values, value])
  }

  const remove = (value: string) => onChange(values.filter((v) => v !== value))

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      add(draft)
    } else if (e.key === 'Backspace' && draft === '' && values.length > 0) {
      remove(values[values.length - 1])
    }
  }

  return (
    <div className="tag-input">
      {values.map((value) => (
        <span className="tag" key={value}>
          {value}
          <button
            type="button"
            className="tag__remove"
            aria-label={`Remove ${value}`}
            onClick={() => remove(value)}
          >
            ×
          </button>
        </span>
      ))}
      <input
        id={id}
        className="tag-input__field"
        value={draft}
        placeholder={values.length === 0 ? placeholder : undefined}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => add(draft)}
      />
    </div>
  )
}

export default TagInput
