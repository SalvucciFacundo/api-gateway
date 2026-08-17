import type { BucketStatus } from '../api'

export function RateLimitBar({ label, status }: { label: string; status: BucketStatus }) {
  const percentage = status.limit > 0 ? Math.max(0, Math.min(100, (status.remaining / status.limit) * 100)) : 0
  return (
    <section className="limit-card">
      <div className="split"><span>{label}</span><strong>{Math.floor(status.remaining)} / {status.limit}</strong></div>
      <div className="limit-track" role="progressbar" aria-label={`${label} remaining`} aria-valuenow={status.remaining} aria-valuemin={0} aria-valuemax={status.limit}>
        <span style={{ width: `${percentage}%` }} />
      </div>
      <small>Reset {new Date(status.reset * 1000).toLocaleTimeString()}</small>
    </section>
  )
}
