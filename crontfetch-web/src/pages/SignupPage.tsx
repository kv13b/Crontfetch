import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Link, Navigate, useNavigate } from 'react-router-dom'
import PasswordInput from '../components/PasswordInput'
import { clearError, signupUser } from '../store/auth'
import { useAppDispatch, useAppSelector } from '../store/hooks'

function SignupPage() {
  const dispatch = useAppDispatch()
  const navigate = useNavigate()
  const { token, loading, error } = useAppSelector((s) => s.auth)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  // Don't carry an error over from the login page.
  useEffect(() => {
    dispatch(clearError())
  }, [dispatch])

  if (token) return <Navigate to="/dashboard" replace />

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const result = await dispatch(signupUser({ name, email, password }))
    if (signupUser.fulfilled.match(result)) navigate('/dashboard')
  }

  return (
    <section className="auth">
      <div className="card auth__card">
        <div className="auth__header">
          <h1>Create your account</h1>
          <p>Start tracking career pages in under a minute.</p>
        </div>

        <form className="form" onSubmit={onSubmit}>
          <div className="form-field">
            <label className="form-label" htmlFor="name">
              Name
            </label>
            <input
              id="name"
              className="input"
              autoComplete="name"
              placeholder="Your name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </div>

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
              autoComplete="new-password"
              placeholder="At least 8 characters"
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
            <span className="form-hint">Use 8 or more characters.</span>
          </div>

          {error && (
            <p className="alert alert--error" role="alert">
              {error}
            </p>
          )}

          <button type="submit" className="btn btn--primary btn--lg btn--block" disabled={loading}>
            {loading && <span className="spinner" aria-hidden="true" />}
            {loading ? 'Creating account…' : 'Create account'}
          </button>
        </form>

        <p className="auth__footer">
          Already have an account? <Link to="/login">Log in</Link>
        </p>
      </div>
    </section>
  )
}

export default SignupPage
