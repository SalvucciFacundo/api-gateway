import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, decodeClaims, tokens, type Limits, type User } from '../api'
import { HealthStatus } from '../components/HealthStatus'
import { RateLimitBar } from '../components/RateLimitBar'

export function ExplorerPage() {
  const navigate = useNavigate(); const [user, setUser] = useState<User>(); const [limits, setLimits] = useState<Limits>(); const [error, setError] = useState(''); const claims = decodeClaims(tokens.get().access)
  useEffect(() => { void Promise.all([api.me(), api.limits()]).then(([nextUser, nextLimits]) => { setUser(nextUser); setLimits(nextLimits) }).catch(err => { tokens.clear(); setError(err instanceof Error ? err.message : 'Session expired'); navigate('/') }) }, [navigate])
  function signOut() { tokens.clear(); navigate('/') }
  return <main className="explorer"><header className="topbar"><div className="brand-mark">API GATEWAY<span>/</span></div><button className="quiet" onClick={signOut}>Sign out</button></header><div className="content"><div className="page-heading"><div><p className="eyebrow">EXPLORER</p><h1>Gateway session</h1></div><span className="live-dot">LIVE</span></div>{error && <p className="error">{error}</p>}<div className="grid"><section className="panel identity"><p className="eyebrow">IDENTITY</p><h2>{user?.email ?? 'Loading...'}</h2><dl><dt>User ID</dt><dd>{user?.id ?? '...'}</dd><dt>Issuer</dt><dd>{String(claims.iss ?? '...')}</dd><dt>Token type</dt><dd>{String(claims.type ?? '...')}</dd><dt>Expires</dt><dd>{claims.exp ? new Date(Number(claims.exp) * 1000).toLocaleString() : '...'}</dd></dl></section><HealthStatus /></div><section className="panel limits"><div className="split"><div><p className="eyebrow">RATE LIMITS</p><h2>Request budget</h2></div><span className="unit">TOKENS / MINUTE</span></div>{limits ? <div className="limit-grid"><RateLimitBar label="Client IP" status={limits.ip} /><RateLimitBar label="Authenticated user" status={limits.user} /></div> : <p className="muted">Loading rate limit status...</p>}</section></div></main>
}
