import FilterBar from '../components/FilterBar'
import GoOnlineToggle from '../components/GoOnlineToggle'
import PagePlaceholder from '../components/PagePlaceholder'

function NearbyPage() {
  return (
    <PagePlaceholder
      title="Jobs around me"
      description="Companies near your location, with a Go online switch for match alerts."
    >
      <GoOnlineToggle />
      <FilterBar />
    </PagePlaceholder>
  )
}

export default NearbyPage
