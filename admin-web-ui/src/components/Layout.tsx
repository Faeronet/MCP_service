import { Outlet, Link, useNavigate, useLocation } from 'react-router-dom'

export function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const logout = () => {
    localStorage.removeItem('token')
    window.dispatchEvent(new Event('auth-change'))
    navigate('/login')
  }

  const nav = [
    { to: '/docs', label: 'Docs' },
    { to: '/jobs', label: 'Jobs' },
    { to: '/logs', label: 'Logs' },
    { to: '/grafana', label: 'Grafana' },
  ]

  return (
    <div className="app-layout">
      <header className="top-bar">
        <h1>Admin</h1>
      </header>
      <div className="middle">
        <aside className="sidebar">
          <nav>
            <ul>
              {nav.map(({ to, label }) => (
                <li key={to}>
                  <Link to={to} className={location.pathname === to ? 'active' : ''}>
                    {label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
          <div className="logout-wrap">
            <button type="button" className="btn-logout" onClick={logout}>
              Logout
            </button>
          </div>
        </aside>
        <main className="main-column">
          <Outlet />
        </main>
      </div>
      <footer className="bottom-bar" aria-hidden="true" />
    </div>
  )
}
