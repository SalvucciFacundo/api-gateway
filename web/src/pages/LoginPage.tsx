import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, tokens } from '../api'

export function LoginPage() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try { tokens.set(await api.login(email, password)); navigate('/explorer') } catch (err) { setError(err instanceof Error ? err.message : 'Unable to log in') } finally { setBusy(false) }
  }
  return <main className="auth-layout"><div className="brand-mark">API GATEWAY<span>/</span></div><form className="panel auth-panel" onSubmit={submit}><p className="eyebrow">ACCESS</p><h1>Sign in</h1><label>Email<input type="email" value={email} onChange={e => setEmail(e.target.value)} required autoComplete="email" /></label><label>Password<input type="password" value={password} onChange={e => setPassword(e.target.value)} required autoComplete="current-password" /></label>{error && <p className="error">{error}</p>}<button disabled={busy}>{busy ? 'Signing in...' : 'Sign in'}</button><p className="switch">No account? <Link to="/register">Create one</Link></p></form></main>
}
