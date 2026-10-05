import { Link } from 'react-router-dom'

// Placeholder marketing content; swap in real copy and numbers later.
const feedItems = [
  { company: 'Linear', title: 'Senior Fullstack Engineer', meta: 'Remote · Europe · 2 min ago' },
  { company: 'Stripe', title: 'Software Engineer, Payments', meta: 'Bangalore · 14 min ago' },
  { company: 'Cloudflare', title: 'Systems Engineer', meta: 'Remote · 38 min ago' },
]

const stats = [
  { value: '9', label: 'Career platforms' },
  { value: '< 1h', label: 'From posting to alert' },
  { value: '0', label: 'Pages to refresh by hand' },
  { value: '24/7', label: 'Background checks' },
]

const features = [
  {
    icon: '🎯',
    title: 'Precise filters',
    text: 'Match on role keywords, locations and experience range so only relevant openings reach you.',
  },
  {
    icon: '⚡',
    title: 'Hourly sync',
    text: 'Every tracked career page is checked in the background, so new roles surface within the hour.',
  },
  {
    icon: '📬',
    title: 'Telegram alerts',
    text: 'Get a message the moment a matching job goes live, with a direct link to apply.',
  },
  {
    icon: '🔍',
    title: 'Auto platform detection',
    text: 'Paste a careers URL and CronFetch works out whether it runs on Greenhouse, Lever, Workday and more.',
  },
  {
    icon: '🧹',
    title: 'No backlog spam',
    text: 'The first sync records existing jobs quietly. You only hear about postings that are genuinely new.',
  },
  {
    icon: '🔒',
    title: 'Private by default',
    text: 'Your tracked companies and filters are yours alone, behind your own account.',
  },
]

const steps = [
  {
    title: 'Add a company',
    text: 'Paste the career page URL of a company you want to work at.',
  },
  {
    title: 'Set your filters',
    text: 'Choose the roles, locations and experience range you care about.',
  },
  {
    title: 'Get notified',
    text: 'CronFetch checks every hour and pings you when a match appears.',
  },
]

const platforms = [
  'Greenhouse',
  'Lever',
  'Workday',
  'Ashby',
  'SmartRecruiters',
  'Workable',
  'Recruitee',
  'TalentBrew',
  'BeeSite',
]

function LandingPage() {
  return (
    <>
      <section className="hero">
        <div className="container hero__inner">
          <div>
            <span className="badge">New · Experience-range filtering</span>
            <h1 className="hero__title">
              Never miss the job <span className="text-gradient">you're waiting for</span>
            </h1>
            <p className="hero__subtitle">
              CronFetch watches the career pages of companies you love and alerts you the moment
              a role that fits you goes live. No more refreshing tabs.
            </p>
            <div className="hero__actions">
              <Link to="/signup" className="btn btn--primary btn--lg">
                Start tracking free
              </Link>
              <Link to={{ pathname: '/', hash: '#how-it-works' }} className="btn btn--secondary btn--lg">
                See how it works
              </Link>
            </div>
            <p className="hero__note">Free for personal use · No credit card needed</p>
          </div>

          <div className="feed" aria-hidden="true">
            <div className="feed__header">
              <span>New matches</span>
              <span className="badge badge--success">● Live</span>
            </div>
            <ul className="feed__list">
              {feedItems.map((item) => (
                <li key={item.title} className="feed__item">
                  <span className="feed__avatar">{item.company[0]}</span>
                  <div className="feed__body">
                    <p className="feed__title">{item.title}</p>
                    <p className="feed__meta">
                      {item.company} · {item.meta}
                    </p>
                  </div>
                  <span className="badge">New</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <div className="container">
        <dl className="stats">
          {stats.map((stat) => (
            <div key={stat.label}>
              <dt className="stats__value">{stat.value}</dt>
              <dd className="stats__label">{stat.label}</dd>
            </div>
          ))}
        </dl>
      </div>

      <section id="features" className="section">
        <div className="container">
          <div className="section-header">
            <span className="eyebrow">Features</span>
            <h2>Everything you need to land the role first</h2>
            <p>Set it up once, then let CronFetch do the watching for you.</p>
          </div>
          <div className="features">
            {features.map((feature) => (
              <article key={feature.title} className="card feature">
                <div className="feature__icon" aria-hidden="true">
                  {feature.icon}
                </div>
                <h3>{feature.title}</h3>
                <p>{feature.text}</p>
              </article>
            ))}
          </div>
        </div>
      </section>

      <section id="how-it-works" className="section section--subtle">
        <div className="container">
          <div className="section-header">
            <span className="eyebrow">How it works</span>
            <h2>Up and running in three steps</h2>
          </div>
          <ol className="steps">
            {steps.map((step) => (
              <li key={step.title} className="step">
                <h3>{step.title}</h3>
                <p>{step.text}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section id="platforms" className="section">
        <div className="container">
          <div className="section-header">
            <span className="eyebrow">Platforms</span>
            <h2>Works with the career sites companies use</h2>
            <p>Most job boards are detected automatically from the URL you paste.</p>
          </div>
          <ul className="platforms">
            {platforms.map((name) => (
              <li key={name} className="platform-chip">
                {name}
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <div className="cta">
            <h2>Your next role could be posted today</h2>
            <p>Create an account, add your dream companies, and let CronFetch keep watch.</p>
            <Link to="/signup" className="btn btn--primary btn--lg">
              Create your free account
            </Link>
          </div>
        </div>
      </section>
    </>
  )
}

export default LandingPage
