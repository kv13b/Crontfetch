import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { logout } from '../store/auth'
import { useAppDispatch, useAppSelector } from '../store/hooks'

function ProfileMenu() {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const user = useAppSelector((s) => s.auth.user)
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  // Close on a click outside the menu or on Escape.
  useEffect(() => {
    if (!open) return

    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }

    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open])

  const handleLogout = () => {
    setOpen(false)
    dispatch(logout())
    navigate('/')
  }

  // The user object isn't restored after a page reload (only the token is),
  // so fall back to a generic label until it is.
  const name = user?.name ?? 'Profile'
  const initial = name.charAt(0).toUpperCase()

  return (
    <div className="profile-menu" ref={rootRef}>
      <button
        type="button"
        className="profile-menu__trigger"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <span className="profile-menu__avatar" aria-hidden="true">
          {initial}
        </span>
        <span className="profile-menu__name">{name}</span>
        <span className="profile-menu__chevron" aria-hidden="true" />
      </button>

      {open && (
        <div className="profile-menu__panel" role="menu">
          {user && (
            <div className="profile-menu__header">
              <div className="profile-menu__header-name">{user.name}</div>
              <div className="profile-menu__header-email">{user.email}</div>
            </div>
          )}
          <Link
            className="profile-menu__item"
            role="menuitem"
            to="/profile"
            onClick={() => setOpen(false)}
          >
            Profile settings
          </Link>
          <button
            type="button"
            className="profile-menu__item profile-menu__item--danger"
            role="menuitem"
            onClick={handleLogout}
          >
            Log out
          </button>
        </div>
      )}
    </div>
  )
}

export default ProfileMenu
