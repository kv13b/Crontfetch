import type { ReactNode } from 'react'

interface PagePlaceholderProps {
  title: string
  description: string
  // Rendered between the heading and the placeholder card, e.g. a filter bar.
  children?: ReactNode
}

// Stand-in for a screen that is planned but not built yet.
function PagePlaceholder({ title, description, children }: PagePlaceholderProps) {
  return (
    <section className="dashboard">
      <div className="container">
        <div className="dashboard__header">
          <h1>{title}</h1>
          <p>{description}</p>
        </div>

        {children}

        <div className="card empty-state">
          <h2>Coming soon</h2>
          <p>This screen is planned and will be built next.</p>
        </div>
      </div>
    </section>
  )
}

export default PagePlaceholder
