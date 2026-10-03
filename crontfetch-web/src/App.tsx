import AuthPage from './pages/AuthPage'
import { logout } from './store/auth'
import { useAppDispatch, useAppSelector } from './store/hooks'

function App() {
  const dispatch = useAppDispatch()
  const { token, user } = useAppSelector((s) => s.auth)

  if (!token) return <AuthPage />

  return (
    <main>
      <h1>Job Fetcher</h1>
      <p>Signed in{user ? ` as ${user.name}` : ''}</p>
      <button onClick={() => dispatch(logout())}>Log out</button>
    </main>
  )
}

export default App
