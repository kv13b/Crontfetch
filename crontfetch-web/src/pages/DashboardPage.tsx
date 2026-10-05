import { useAppSelector } from '../store/hooks'

function DashboardPage() {
  const user = useAppSelector((s) => s.auth.user)

  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>Welcome{user ? `, ${user.name}` : ''} 👋</h1>
          <p>Here's where your tracked companies and new matches will show up.</p>
        </div>

        <div className="card empty-state">
          <h2>No companies tracked yet</h2>
          <p>Add a career page to start getting alerts for roles that match you.</p>
          <button type="button" className="btn btn--primary" disabled>
            Add company (coming soon)
          </button>
        </div>
      </div>
    </section>
  )
}

export default DashboardPage
