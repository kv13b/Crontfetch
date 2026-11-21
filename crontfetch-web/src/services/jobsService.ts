import { request } from './api'

export interface Job {
  id: string
  title: string
  location: string
  url: string
  // Stated experience range, e.g. "3-5 years", when the listing gives one.
  experience?: string
}

export interface CompanyJobs {
  company: string
  total_scanned: number
  total_matched: number
  warnings?: string[]
  jobs: Job[]
}

// Scrapes the career site live, so it can take 10+ seconds for big sites.
export const fetchCompanyJobs = (token: string | null, companyId: string) =>
  request<CompanyJobs>(`/companies/${encodeURIComponent(companyId)}/jobs`, { token })
