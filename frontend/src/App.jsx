import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import WorkspaceLayout from './pages/WorkspaceLayout'
import './index.css'

function ProtectedRoute({ children }) {
  const isAuthenticated = useAuthStore(s => s.isAuthenticated)
  return isAuthenticated ? children : <Navigate to="/login" replace />
}

function GuestRoute({ children }) {
  const isAuthenticated = useAuthStore(s => s.isAuthenticated)
  return !isAuthenticated ? children : <Navigate to="/" replace />
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<GuestRoute><LoginPage /></GuestRoute>} />
      <Route path="/register" element={<GuestRoute><RegisterPage /></GuestRoute>} />
      <Route path="/*" element={<ProtectedRoute><WorkspaceLayout /></ProtectedRoute>} />
    </Routes>
  )
}

export default App
