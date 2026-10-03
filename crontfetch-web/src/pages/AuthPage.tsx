import { useState } from 'react'
import type { FormEvent } from 'react'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { clearError, loginUser, signupUser } from '../store/auth'

function AuthPage() {
  const dispatch = useAppDispatch()
  const { loading, error } = useAppSelector((s) => s.auth)
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (mode === 'signup') dispatch(signupUser({ name, email, password }))
    else dispatch(loginUser({ email, password }))
  }

  const switchMode = () => {
    dispatch(clearError())
    setMode(mode === 'login' ? 'signup' : 'login')
  }

  return (
    <form onSubmit={onSubmit}>
      <h2>{mode === 'login' ? 'Log in' : 'Sign up'}</h2>
      {mode === 'signup' && (
        <input
          placeholder="Name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
      )}
      <input
        type="email"
        placeholder="Email"
        value={email}
        onChange={(e) => setEmail(e.target.value)}
        required
      />
      <input
        type="password"
        placeholder="Password"
        minLength={8}
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        required
      />
      {error && <p role="alert">{error}</p>}
      <button type="submit" disabled={loading}>
        {loading ? 'Please wait...' : mode === 'login' ? 'Log in' : 'Sign up'}
      </button>
      <button type="button" onClick={switchMode}>
        {mode === 'login' ? 'Need an account? Sign up' : 'Have an account? Log in'}
      </button>
    </form>
  )
}

export default AuthPage
