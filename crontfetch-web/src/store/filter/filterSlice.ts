import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import { emptyFilter } from '../../types/filter'
import type { JobFilter } from '../../types/filter'
import { logout } from '../auth'

const FILTER_KEY = 'jobFilter'

// The filter is saved in the browser for now. When the backend's profile
// endpoints are connected, load and save through them instead.
function loadFilter(): JobFilter {
  try {
    const raw = localStorage.getItem(FILTER_KEY)
    if (raw) {
      // Pick known fields only, so a filter saved by an older version
      // (min/max experience) can't leave stale keys behind.
      const saved = JSON.parse(raw)
      return {
        locations: saved.locations ?? [],
        roles: saved.roles ?? [],
        interests: saved.interests ?? [],
        experienceYears: saved.experienceYears ?? null,
      }
    }
  } catch {
    // storage unavailable or corrupt; start empty
  }
  return emptyFilter
}

function saveFilter(filter: JobFilter | null) {
  try {
    if (filter) localStorage.setItem(FILTER_KEY, JSON.stringify(filter))
    else localStorage.removeItem(FILTER_KEY)
  } catch {
    // storage unavailable; the filter just won't persist across reloads
  }
}

export interface FilterState {
  filter: JobFilter
}

const initialState: FilterState = { filter: loadFilter() }

const filterSlice = createSlice({
  name: 'filter',
  initialState,
  reducers: {
    setFilter(state, action: PayloadAction<JobFilter>) {
      state.filter = action.payload
      saveFilter(action.payload)
    },
  },
  extraReducers: (builder) => {
    // Don't leave one person's filter behind on a shared device.
    builder.addCase(logout, (state) => {
      state.filter = emptyFilter
      saveFilter(null)
    })
  },
})

export const { setFilter } = filterSlice.actions
export default filterSlice.reducer
