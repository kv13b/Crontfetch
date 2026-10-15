import { createAsyncThunk } from '@reduxjs/toolkit'
import { request } from '../../services/api'
import type { RootState } from '../index'

export const addCompany = createAsyncThunk(
  'companies/addCompany',
  async (
    payload: {
      name: string
      career_url: string
      min_experience_years: number | null
      max_experience_years: number | null
      roles: string[]
      locations: string[]
    },
    { rejectWithValue, getState }
  ) => {
    try {
      const token = (getState() as RootState).auth.token
      const response = await request('/companies', {
        method: 'POST',
        body: payload,
        token,
      })
      return response
    } catch (e: any) {
      if (e instanceof Error) {
        return rejectWithValue(e.message)
      }
      return rejectWithValue('Unknown error')
    }
  }
)

export const fetchCompanies = createAsyncThunk(
  'companies/fetchCompanies',
  async (_, { rejectWithValue, getState }) => {
    try {
      const token = (getState() as RootState).auth.token
      const response = await request('/companies', {
        method: 'GET',
        token,
      })
      return response
    } catch (e: any) {
      if (e instanceof Error) {
        return rejectWithValue(e.message)
      }
      return rejectWithValue('Unknown error')
    }
  }
)
export const updateCompany = createAsyncThunk(
  'companies/updateCompany',
  async (
    {
      id,
      payload,
    }: {
      id: string
      payload: {
        name?: string
        career_url?: string
        min_experience_years?: number | null
        max_experience_years?: number | null
        roles?: string[]
        locations?: string[]
      }
    },
    { rejectWithValue, getState }
  ) => {
    try {
      const token = (getState() as RootState).auth.token
      const response = await request(`/companies/${id}`, {
        method: 'PATCH',
        body: payload,
        token,
      })
      return response
    } catch (e: any) {
      if (e instanceof Error) {
        return rejectWithValue(e.message)
      }
      return rejectWithValue('Unknown error')
    }
  }
)