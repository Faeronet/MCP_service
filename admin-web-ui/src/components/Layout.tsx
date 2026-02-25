import { Outlet, Link, useNavigate } from 'react-router-dom'

export function Layout() {
  const navigate = useNavigate()
  const logout = () => {
    localStorage.removeItem('token')
    window.dispatchEvent(new Event('auth-change'))
    navigate('/login')
  }
  return (
    <div style={{ display: 'flex', minHeight: '100vh' }}>
      <nav style={{ width: 200, padding: 24, borderRight: '1px solid #27272a', background: '#18181b' }}>
        <h2 style={{ margin: '0 0 24px 0', fontSize: 18 }}>Admin</h2>
        <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
          <li style={{ marginBottom: 8 }}><Link to="/docs">Docs</Link></li>
          <li style={{ marginBottom: 8 }}><Link to="/jobs">Jobs</Link></li>
          <li style={{ marginBottom: 8 }}><Link to="/logs">Logs</Link></li>
          <li style={{ marginBottom: 8 }}><Link to="/grafana">Grafana</Link></li>
        </ul>
        <button onClick={logout} style={{ marginTop: 24, padding: '8px 16px', cursor: 'pointer' }}>Logout</button>
      </nav>
      <main style={{ flex: 1, padding: 24 }}>
        <Outlet />
      </main>
    </div>
  )
}
