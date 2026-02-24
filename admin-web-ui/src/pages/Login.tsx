import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login } from '../api'

export function Login() {
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [err, setErr] = useState('')
  const navigate = useNavigate()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErr('')
    try {
      const { token } = await login(user, pass)
      localStorage.setItem('token', token)
      navigate('/docs')
    } catch {
      setErr('Invalid credentials')
    }
  }

  return (
    <div style={{ maxWidth: 400, margin: '80px auto', padding: 24 }}>
      <h1>Admin Login</h1>
      <form onSubmit={submit}>
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 4 }}>Username</label>
          <input type="text" value={user} onChange={e => setUser(e.target.value)} required style={{ width: '100%', padding: 8 }} />
        </div>
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', marginBottom: 4 }}>Password</label>
          <input type="password" value={pass} onChange={e => setPass(e.target.value)} required style={{ width: '100%', padding: 8 }} />
        </div>
        {err && <p style={{ color: '#f87171', marginBottom: 16 }}>{err}</p>}
        <button type="submit" style={{ padding: '10px 24px', cursor: 'pointer' }}>Login</button>
      </form>
    </div>
  )
}
