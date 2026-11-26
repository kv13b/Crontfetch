import { useEffect, useState } from 'react'
import {
  createTelegramLink,
  disconnectTelegram,
  fetchTelegramStatus,
} from '../services/telegramService'
import type { TelegramLink } from '../services/telegramService'
import { useAppSelector } from '../store/hooks'

type Status = 'loading' | 'disconnected' | 'waiting' | 'connected' | 'error'

// How often to check whether the user has tapped Start in Telegram.
const POLL_MS = 3000

// Connect, wait for Start, connected. The user never sees a token or chat ID:
// the link carries a one-time code that tells the server who tapped Start.
function TelegramConnect() {
  const token = useAppSelector((s) => s.auth.token)
  const [status, setStatus] = useState<Status>('loading')
  const [link, setLink] = useState<TelegramLink | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    let cancelled = false
    fetchTelegramStatus(token)
      .then((s) => {
        if (!cancelled) setStatus(s.connected ? 'connected' : 'disconnected')
      })
      .catch((e: Error) => {
        if (cancelled) return
        setMessage(e.message)
        setStatus('error')
      })
    return () => {
      cancelled = true
    }
  }, [token, reload])

  // While waiting, check until the user taps Start or the link expires.
  useEffect(() => {
    if (status !== 'waiting' || !link) return
    const expiresAt = new Date(link.expires_at).getTime()
    const timer = setInterval(() => {
      if (Date.now() > expiresAt) {
        setStatus('disconnected')
        setMessage('That link expired. Tap Connect Telegram to get a new one.')
        return
      }
      fetchTelegramStatus(token)
        .then((s) => {
          if (s.connected) setStatus('connected')
        })
        .catch(() => {}) // a missed check is retried on the next tick
    }, POLL_MS)
    return () => clearInterval(timer)
  }, [status, link, token])

  const connect = async () => {
    setBusy(true)
    setMessage(null)
    try {
      const l = await createTelegramLink(token)
      setLink(l)
      setStatus('waiting')
      // Some browsers block this after an await, so the page also shows the link.
      window.open(l.url, '_blank', 'noopener')
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not create the link')
    } finally {
      setBusy(false)
    }
  }

  const disconnect = async () => {
    if (!window.confirm('Stop sending job alerts to Telegram?')) return
    setBusy(true)
    setMessage(null)
    try {
      await disconnectTelegram(token)
      setStatus('disconnected')
    } catch (e) {
      setMessage(e instanceof Error ? e.message : 'Could not disconnect')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="card telegram-connect">
      <h2>Telegram alerts</h2>

      {status === 'loading' && <p role="status">Checking your Telegram connection…</p>}

      {status === 'error' && (
        <>
          <p className="alert alert--error" role="alert">
            Couldn't check your Telegram connection: {message}
          </p>
          <div>
            <button
              type="button"
              className="btn btn--secondary"
              onClick={() => {
                setStatus('loading')
                setReload((n) => n + 1)
              }}
            >
              Retry
            </button>
          </div>
        </>
      )}

      {status === 'disconnected' && (
        <>
          <p>Get a message on your phone the moment a matching job is posted.</p>
          {message && (
            <p className="alert alert--error" role="alert">
              {message}
            </p>
          )}
          <div className="telegram-connect__actions">
            <button type="button" className="btn btn--primary" onClick={connect} disabled={busy}>
              {busy ? 'Opening…' : 'Connect Telegram'}
            </button>
            <a
              href="https://telegram.org/apps"
              target="_blank"
              rel="noreferrer"
              className="telegram-connect__hint"
            >
              Don't have Telegram? Get it here
            </a>
          </div>
        </>
      )}

      {status === 'waiting' && link && (
        <>
          <p role="status">
            <span className="spinner" aria-hidden="true" /> In Telegram, tap <strong>Start</strong>.
            This page updates by itself.
          </p>
          <div className="telegram-connect__actions">
            <a href={link.url} target="_blank" rel="noreferrer" className="btn btn--primary">
              Open Telegram
            </a>
            <button
              type="button"
              className="btn btn--ghost"
              onClick={() => setStatus('disconnected')}
            >
              Cancel
            </button>
          </div>
        </>
      )}

      {status === 'connected' && (
        <>
          <p className="alert alert--success" role="status">
            ✅ Connected. New matching jobs are sent to your Telegram.
          </p>
          {message && (
            <p className="alert alert--error" role="alert">
              {message}
            </p>
          )}
          <div>
            <button type="button" className="btn btn--danger" onClick={disconnect} disabled={busy}>
              {busy ? 'Disconnecting…' : 'Disconnect'}
            </button>
          </div>
        </>
      )}
    </div>
  )
}

export default TelegramConnect
