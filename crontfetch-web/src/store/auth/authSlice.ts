import { createSlice } from '@reduxjs/toolkit'
import type { User } from '../../types/auth'
import { loginUser, signupUser } from './authThunks'

const TOKEN_KEY = 'token'

function loadToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

function saveToken(token: string | null) {
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch {
    // storage unavailable; token just won't persist across reloads
  }
}

export interface AuthState {
  user: User | null
  token: string | null
  loading: boolean
  error: string | null
}

const initialState: AuthState = {
  user: null,
  token: loadToken(),
  loading: false,
  error: null,
}

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    logout(state) {
      state.user = null
      state.token = null
      state.error = null
      saveToken(null)
    },
    clearError(state) {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    for (const thunk of [signupUser, loginUser]) {
      builder
        .addCase(thunk.pending, (state) => {
          state.loading = true
          state.error = null
        })
        .addCase(thunk.fulfilled, (state, action) => {
          state.loading = false
          state.user = action.payload.user
          state.token = action.payload.token
          saveToken(action.payload.token)
        })
        .addCase(thunk.rejected, (state, action) => {
          state.loading = false
          state.error = (action.payload as string) ?? action.error.message ?? null
        })
    }
  },
})

export const { logout, clearError } = authSlice.actions
export default authSlice.reducer
