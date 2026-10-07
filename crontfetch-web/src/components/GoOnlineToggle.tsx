import { useState } from 'react'
import { useAppDispatch, useAppSelector } from '../store/hooks'
import { setOnline } from '../store/presence'
import { isFilterEmpty } from '../types/filter'

function GoOnlineToggle() {
  const dispatch = useAppDispatch()
  const online = useAppSelector((s) => s.presence.online)
  const filter = useAppSelector((s) => s.filter.filter)
  const [blocked, setBlocked] = useState(false)

  const noFilter = isFilterEmpty(filter)
  // Show the warning only while it still applies, so it disappears as soon
  // as the user sets a filter.
  const showBlocked = blocked && noFilter && !online

  const toggle = () => {
    if (online) {
      dispatch(setOnline(false))
      return
    }
    // Alerts need something to match against, so require a filter first.
    if (noFilter) {
      setBlocked(true)
      return
    }
    setBlocked(false)
    dispatch(setOnline(true))
  }

  return (
    <div className={`card online-card${online ? ' online-card--on' : ''}`}>
      <div className="online-card__row">
        <div>
          <h2 className="online-card__title">
            <span className="online-card__dot" aria-hidden="true" />
            {online ? 'You are online' : 'You are offline'}
          </h2>
          <p className="online-card__text">
            {online
              ? 'Alerts on for matching jobs near you.'
              : 'Go online for alerts on matching jobs near you.'}
          </p>
        </div>

        <button
          type="button"
          role="switch"
          aria-checked={online}
          aria-label={online ? 'Go offline' : 'Go online'}
          className={`switch${online ? ' switch--on' : ''}`}
          onClick={toggle}
        >
          <span className="switch__thumb" />
        </button>
      </div>

      {showBlocked && (
        <p className="alert alert--error" role="alert">
          Set at least one filter below before going online.
        </p>
      )}

      <p className="form-hint">Tracking and alerts are not connected yet.</p>
    </div>
  )
}

export default GoOnlineToggle
