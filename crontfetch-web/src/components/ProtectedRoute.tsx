import { Navigate, Outlet } from 'react-router-dom'
import { useAppSelector } from '../store/hooks'

function ProtectedRoute() {
  const token = useAppSelector((s) => s.auth.token)
  return token ? <Outlet /> : <Navigate to="/login" replace />
}

export default ProtectedRoute
