import { useAuthStore } from '../store'
import { Link } from 'react-router-dom'

export default function Navbar() {
  const { user, logout } = useAuthStore()

  return (
    <nav className="navbar">
      <Link to="/" className="navbar-brand">
        <div className="navbar-logo">N</div>
        <span className="navbar-title">NGAC Platform</span>
      </Link>
      <div className="navbar-user">
        <div className="navbar-info">
          <div className="navbar-username">{user?.username}</div>
          <div className="navbar-dept">{user?.department} · {user?.company}</div>
        </div>
        <button className="btn btn-secondary btn-sm" onClick={logout}>
          Logout
        </button>
      </div>
    </nav>
  )
}
