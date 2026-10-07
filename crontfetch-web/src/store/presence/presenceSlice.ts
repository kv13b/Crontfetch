import { createSlice } from '@reduxjs/toolkit'
import type { PayloadAction } from '@reduxjs/toolkit'
import { logout } from '../auth'

// Whether the user has switched on location tracking and match alerts.
// Deliberately not saved across reloads: tracking stops when the page does,
// so the user starts offline and chooses to go online again.
export interface PresenceState {
  online: boolean
}

const initialState: PresenceState = { online: false }

const presenceSlice = createSlice({
  name: 'presence',
  initialState,
  reducers: {
    setOnline(state, action: PayloadAction<boolean>) {
      state.online = action.payload
    },
  },
  extraReducers: (builder) => {
    builder.addCase(logout, (state) => {
      state.online = false
    })
  },
})

export const { setOnline } = presenceSlice.actions
export default presenceSlice.reducer
