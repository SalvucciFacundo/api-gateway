import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api'

export function RegisterPage() {
  const navigate = useNavigate(); const [email, setEmail] = useState(''); const [password, setPassword] = useState(''); const [error, setError] = useState(''); const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) { event.preventDefault(); setBusy(true); setError(''); try { await api.register(email, password); navigate('/', { state: { registered: true } }) } catch (err) { setError(err instanceof Error ? err.message : 'Unable to register') } finally { setBusy(false) } }
  return <main className="auth-layout"><div className="brand-mark">API GATEWAY<span>/</span></div><form className="panel auth-panel" onSubmit={submit}><p className="eyebrow">NEW USER</p><h1>Create account</h1><label>Email<input type="email" value={email} onChange={e => setEmail(e.target.value)} required autoComplete="email" /></label><label>Password<input type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} autoComplete="new-password" /></label><p className="hint">Use at least 8 characters.</p>{error && <p className="error">{error}</p>}<button disabled={busy}>{busy ? 'Creating...' : 'Create account'}</button><p className="switch">Already registered? <Link to="/">Sign in</Link></p></form></main>
}
