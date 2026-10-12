import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { fetchCompanies } from '../store/companies/companiesThunks'

function DashboardPage() {
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const { loading, error, companies } = useAppSelector((s) => s.companies)

  useEffect(() => {
    dispatch(fetchCompanies())
  }, [dispatch])

  if (loading) {
    return (
      <section className="dashboard">
        <div className="container">
          <div className="dashboard__header">
            <h1>Welcome</h1>
            <p>Loading your companies...</p>
          </div>
        </div>
      </section>
    )
  }

  if (error) {
    return (
      <section className="dashboard">
        <div className="container">
          <div className="dashboard__header">
            <h1>Welcome</h1>
            <p>Error loading companies.</p>
          </div>
          <div className="card">
            <h2>Error</h2>
            <p>{error}</p>
          </div>
        </div>
      </section>
    )
  }

  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>Welcome{companies.length > 0 ? `, you have ${companies.length} company${companies.length > 1 ? 'ies' : ''} tracked` : ''} 👋</h1>
          <p>{companies.length === 0 ? 'No companies tracked yet. Add a career page to start getting alerts for roles that match you.' : 'Here\'s where your tracked companies and new matches will show up.'}</p>
        </div>

        <div className="card">
          {companies.length === 0 ? (
            <>
              <h2>No companies tracked yet</h2>
              <p>Add a career page to start getting alerts for roles that match you.</p>
              <button type="button" className="btn btn--primary" onClick={() => navigate('/add-jobs')}>
                Add company
              </button>
            </>
          ) : (
            <>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--space-4)' }}>
                <h2>Tracked Companies</h2>
                <button type="button" className="btn btn--primary" onClick={() => navigate('/add-jobs')}>
                  Add another company
                </button>
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(250px, 1fr))', gap: 'var(--space-4)' }}>
                {companies.map((company) => (
                  <div key={company.id} className="card">
                    <h3>{company.name}</h3>
                    <p className="company-url">{company.career_url}</p>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  )
}

export default DashboardPage