import { request } from './api'

export interface SyncResult {
  companies: number
  new_jobs: number
  failed: { company_id: string; name: string; error: string }[]
}

// Checks every tracked company now. Companies run one after another on the
// server, so this can take a minute or more.
export const runSyncNow = (token: string | null) =>
  request<SyncResult>('/run-now', { method: 'POST', token })
