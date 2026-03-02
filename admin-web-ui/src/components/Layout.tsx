import { Outlet, Link, useNavigate, useLocation } from 'react-router-dom'
import { FileText, Briefcase, ScrollText, MessageCircle, BarChart3, Gauge, LogOut } from 'lucide-react'

export function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const logout = () => {
    localStorage.removeItem('token')
    window.dispatchEvent(new Event('auth-change'))
    navigate('/login')
  }

  const nav = [
    { to: '/docs', label: 'Docs', icon: FileText },
    { to: '/jobs', label: 'Jobs', icon: Briefcase },
    { to: '/logs', label: 'Logs', icon: ScrollText },
    { to: '/chat-log', label: 'Chat Log', icon: MessageCircle },
    { to: '/monitor', label: 'Monitor', icon: Gauge },
    { to: '/grafana', label: 'Grafana', icon: BarChart3 },
  ]

  return (
    <div className="app-layout">
      <div className="middle">
        <aside className="sidebar">
          <div className="sidebar-title">Admin</div>
          <nav>
            <ul>
              {nav.map(({ to, label, icon: Icon }) => (
                <li key={to}>
                  <Link to={to} className={location.pathname === to ? 'active' : ''}>
                    <Icon className="sidebar-nav-icon" aria-hidden />
                    {label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <div className="logout-wrap">
            <button type="button" className="btn-logout" onClick={logout}>
              <LogOut className="sidebar-nav-icon" aria-hidden />
              Logout
            </button>
          </div>
        </aside>
        <main className="main-column">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
