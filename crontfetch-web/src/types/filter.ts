// What the user is looking for. A job matches when any one field fits,
// but at least one field must be set before going online for alerts.
export interface JobFilter {
  locations: string[]
  roles: string[]
  interests: string[]
  // The user's own years of experience, checked against a job's stated range.
  experienceYears: number | null
}

export const emptyFilter: JobFilter = {
  locations: [],
  roles: [],
  interests: [],
  experienceYears: null,
}

export function isFilterEmpty(f: JobFilter): boolean {
  return (
    f.locations.length === 0 &&
    f.roles.length === 0 &&
    f.interests.length === 0 &&
    f.experienceYears === null
  )
}
