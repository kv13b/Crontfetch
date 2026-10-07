import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import PasswordInput from '../components/PasswordInput'
import { clearError, loginUser } from '../store/auth'
import { useAppDispatch, useAppSelector } from '../store/hooks'

function LoginPage() {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const { token, loading, error } = useAppSelector((s) => s.auth)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  // Don't carry an error over from the signup page.
  useEffect(() => {
    dispatch(clearError())
  }, [dispatch])

  if (token) return <Navigate to="/dashboard" replace />

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const result = await dispatch(loginUser({ email, password }))
    if (loginUser.fulfilled.match(result)) navigate('/dashboard')
  }

  return (
    <section className="auth">
      <div className="card auth__card">
        <div className="auth__header">
          <h1>Welcome back</h1>
          <p>Log in to see your tracked companies.</p>
        </div>

        <form className="form" onSubmit={onSubmit}>
          <div className="form-field">
            <label className="form-label" htmlFor="email">
              Email
            </label>
            <input
              id="email"
              className="input"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>

          <div className="form-field">
            <label className="form-label" htmlFor="password">
              Password
            </label>
            <PasswordInput
              id="password"
              autoComplete="current-password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          {error && (
            <p className="alert alert--error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn--primary btn--lg btn--block" disabled={loading}>
            {loading && <span className="spinner" aria-hidden="true" />}
            {loading ? 'Logging in…' : 'Log in'}
          </button>
        </form>

        <p className="auth__footer">
          Don't have an account? <Link to="/signup">Sign up</Link>
        </p>
      </div>
    </section>
  )
}

export default LoginPage
