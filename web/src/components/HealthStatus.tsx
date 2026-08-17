import { useEffect, useState } from 'react'
import { api } from '../api'

export function HealthStatus() {
  const [health, setHealth] = useState('checking')
  const [ready, setReady] = useState('checking')
  useEffect(() => {
    void Promise.allSettled([api.health(), api.ready()]).then(([healthResult, readyResult]) => {
      setHealth(healthResult.status === 'fulfilled' ? healthResult.value.status : 'offline')
      setReady(readyResult.status === 'fulfilled' ? readyResult.value.status : 'not ready')
    })
  }, [])
  return <section className="health-panel"><div className="split"><span>Gateway health</span><strong className={health === 'ok' ? 'good' : ''}>{health}</strong></div><div className="split"><span>Store readiness</span><strong className={ready === 'ready' ? 'good' : ''}>{ready}</strong></div></section>
}
