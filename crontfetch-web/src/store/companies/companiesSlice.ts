import { createSlice } from '@reduxjs/toolkit'
import { addCompany, fetchCompanies, updateCompany } from './companiesThunks'

export interface CompaniesState {
  loading: boolean
  error: string | null
  companies: any[] // or define a proper type later
}

const initialState: CompaniesState = {
  loading: false,
  error: null,
  companies: [],
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
      .addCase(fetchCompanies.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(fetchCompanies.fulfilled, (state, action) => {
        state.loading = false
        state.error = null
        state.companies = action.payload as any[]
      })
      .addCase(fetchCompanies.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
      // Update Company (PATCH /companies/{id})
      .addCase(updateCompany.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(updateCompany.fulfilled, (state, action) => {
        state.loading = false
        state.error = null
        // Assuming action.payload returns the updated company object
        const updated = action.payload as any
        const index = state.companies.findIndex((c) => c.id === updated.id)
        if (index !== -1) {
          state.companies[index] = updated
        }
      })
      .addCase(updateCompany.rejected, (state, action) => {
        state.loading = false
        state.error = action.payload as string
      })
  },
})

export default companiesSlice.reducer