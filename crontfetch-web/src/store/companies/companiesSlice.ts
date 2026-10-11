import { createSlice } from '@reduxjs/toolkit'
import { addCompany } from './companiesThunks'

export interface CompaniesState {
  loading: boolean
  error: string | null
}

const initialState: CompaniesState = {
  loading: false,
  error: null,
}

const companiesSlice = createSlice({
  name: 'companies',
  initialState,
  reducers: {},
  extraReducers: (builder) => {
    builder
      .addCase(addCompany.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(addCompany.fulfilled, (state) => {
        state.loading = false
        state.error = null
      })
      .addCase(addCompany.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
  },
})

export default companiesSlice.reducer