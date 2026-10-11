import { configureStore } from '@reduxjs/toolkit'
import { authReducer } from './auth'
import { filterReducer } from './filter'
import { presenceReducer } from './presence'
import companiesReducer from './companies/companiesSlice'

export const store = configureStore({
  reducer: {
    auth: authReducer,
    filter: filterReducer,
    presence: presenceReducer,
    companies: companiesReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch