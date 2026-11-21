import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { fetchCompanyJobs } from '../services/jobsService'
import type { CompanyJobs } from '../services/jobsService'
import { useAppSelector } from '../store/hooks'

// Jobs at one tracked company that match its saved filters, read live from
// its career site.
function CompanyJobsPage() {
  const { id = '' } = useParams()
  const token = useAppSelector((s) => s.auth.token)
  const [data, setData] = useState<CompanyJobs | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    fetchCompanyJobs(token, id)
      .then((d) => {
        if (!cancelled) setData(d)
      })
      .catch((e: Error) => {
        if (!cancelled) setError(e.message)
      })
    return () => {
      cancelled = true
    }
  }, [token, id])

  return (
    <section className="dashboard">
      <div className="container">
        <Link to="/dashboard" className="back-link">
          ← Back to dashboard
        </Link>

        {error ? (
          <div className="card">
            <h2>Couldn't load jobs</h2>
            <p className="alert alert--error" role="alert">
              {error}
            </p>
          </div>
        ) : !data ? (
          <div className="card" role="status">
            <p>
              <span className="spinner" aria-hidden="true" /> Checking the career page for
              matching jobs… Large sites can take 10–20 seconds.
            </p>
          </div>
        ) : (
          <>
            <div className="dashboard__header">
              <h1>{data.company}</h1>
              <p>
                {data.total_matched} of {data.total_scanned} open jobs match your filters.
              </p>
            </div>

            {data.warnings?.map((w) => (
              <p key={w} className="alert alert--info">
                {w}
              </p>
            ))}

            {data.jobs.length === 0 ? (
              <div className="card empty-state">
                <h2>No matching jobs right now</h2>
                <p>Try widening this company's roles or locations.</p>
                <Link to="/dashboard" className="btn btn--secondary">
                  Back to dashboard
                </Link>
              </div>
            ) : (
              <ul className="card job-list">
                {data.jobs.map((job) => (
                  <li key={job.id} className="job-list__item">
                    <div>
                      <h3>{job.title}</h3>
                      <p>
                        {job.location || 'Location not listed'}
                        {job.experience && ` · ${job.experience}`}
                      </p>
                    </div>
                    <a
                      href={job.url}
                      target="_blank"
                      rel="noreferrer"
                      className="btn btn--primary btn--sm"
                    >
                      View job ↗
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </section>
  )
}

export default CompanyJobsPage
