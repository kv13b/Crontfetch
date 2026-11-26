import PagePlaceholder from '../components/PagePlaceholder'
import TelegramConnect from '../components/TelegramConnect'

function NotificationsPage() {
  return (
    <PagePlaceholder
      title="Notifications"
      description="Choose where job alerts go. Alerts you may have missed will also appear here, kept for 5 days."
    >
      <TelegramConnect />
    </PagePlaceholder>
  )
}

export default NotificationsPage
