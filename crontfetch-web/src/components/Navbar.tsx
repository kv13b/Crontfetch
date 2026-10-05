import { useState } from 'react'
import type { MouseEvent } from 'react'
import { Link, NavLink, useNavigate } from 'react-router-dom'
import { logout } from '../store/auth'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import Logo from './Logo'

const sectionLinks = [
  { label: 'Features', hash: '#features' },
  { label: 'How it works', hash: '#how-it-works' },
  { label: 'Platforms', hash: '#platforms' },
]

function Navbar() {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const { token, user } = useAppSelector((s) => s.auth)
  const [open, setOpen] = useState(false)

  // Close the mobile menu whenever a link or button inside it is clicked.
  const closeOnNavigate = (e: MouseEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest('a, button')) setOpen(false)
  }

  const handleLogout = () => {
    dispatch(logout())
    navigate('/')
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
            {sectionLinks.map((link) => (
              <li key={link.hash}>
                <Link className="navbar__link" to={{ pathname: '/', hash: link.hash }}>
                  {link.label}
                </Link>
              </li>
            ))}
            {token && (
              <li>
                <NavLink className="navbar__link" to="/dashboard">
                  Dashboard
                </NavLink>
              </li>
            )}
          </ul>

          <div className="navbar__actions">
            {token ? (
              <>
                {user && <span className="navbar__user">Hi, {user.name}</span>}
                <button type="button" className="btn btn--secondary" onClick={handleLogout}>
                  Log out
                </button>
              </>
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
