import { useState } from 'react'
import type { MouseEvent } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { useAppSelector } from '../store/hooks'
import Logo from './Logo'
import ProfileMenu from './ProfileMenu'

const sectionLinks = [
  { label: 'Features', hash: '#features' },
  { label: 'How it works', hash: '#how-it-works' },
  { label: 'Platforms', hash: '#platforms' },
]

// Menu shown once logged in.
const appLinks = [
  { label: 'Dashboard', to: '/dashboard' },
  { label: 'Search jobs', to: '/search' },
  { label: 'Jobs around me', to: '/nearby' },
  { label: 'Notifications', to: '/notifications' },
]

function Navbar() {
  const token = useAppSelector((s) => s.auth.token)
  const [open, setOpen] = useState(false)

  // Close the mobile menu whenever a link or button inside it is clicked,
  // except the profile button, which only opens its own dropdown.
  const closeOnNavigate = (e: MouseEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest('a, button:not([aria-haspopup])')) setOpen(false)
  }

  return (
    <header className={`navbar${open ? ' navbar--open' : ''}`}>
      <nav className="container navbar__inner" aria-label="Main">
        <Logo />

        <button
          type="button"
          className="navbar__toggle"
          aria-label={open ? 'Close menu' : 'Open menu'}
          aria-expanded={open}
          onClick={() => setOpen(!open)}
        >
          <span className="navbar__toggle-bar" />
        </button>

        <div className="navbar__menu" onClick={closeOnNavigate}>
          <ul className="navbar__links">
            {token
              ? appLinks.map((link) => (
                  <li key={link.to}>
                    <NavLink className="navbar__link" to={link.to}>
                      {link.label}
                    </NavLink>
                  </li>
                ))
              : sectionLinks.map((link) => (
                  <li key={link.hash}>
                    <Link className="navbar__link" to={{ pathname: '/', hash: link.hash }}>
                      {link.label}
                    </Link>
                  </li>
                ))}
          </ul>

          <div className="navbar__actions">
            {token ? (
              <ProfileMenu />
            ) : (
              <>
                <Link className="btn btn--ghost" to="/login">
                  Log in
                </Link>
                <Link className="btn btn--primary" to="/signup">
                  Sign up
                </Link>
              </>
            )}
          </div>
        </div>
      </nav>
    </header>
  )
}

export default Navbar
