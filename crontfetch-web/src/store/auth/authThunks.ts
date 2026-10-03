import { createAsyncThunk } from '@reduxjs/toolkit'
import * as authService from '../../services/authService'
import type { LoginPayload, SignupPayload } from '../../types/auth'

const errorMessage = (e: unknown) =>
  e instanceof Error ? e.message : 'Something went wrong'

export const signupUser = createAsyncThunk(
  'auth/signup',
  async (payload: SignupPayload, { rejectWithValue }) => {
    try {
      return await authService.signup(payload)
    } catch (e) {
      return rejectWithValue(errorMessage(e))
    }
  },
)

export const loginUser = createAsyncThunk(
  'auth/login',
  async (payload: LoginPayload, { rejectWithValue }) => {
    try {
      return await authService.login(payload)
    } catch (e) {
      return rejectWithValue(errorMessage(e))
    }
  },
)
