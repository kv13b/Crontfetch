import { Link } from 'react-router-dom'

function Logo() {
  return (
    <Link to="/" className="logo" aria-label="CronFetch home">
      <span className="logo__mark" aria-hidden="true">
        CF
      </span>
      CronFetch
    </Link>
  )
}

export default Logo
