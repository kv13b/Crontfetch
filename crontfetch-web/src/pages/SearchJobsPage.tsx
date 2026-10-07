import FilterBar from '../components/FilterBar'
import PagePlaceholder from '../components/PagePlaceholder'

function SearchJobsPage() {
  return (
    <PagePlaceholder
      title="Search jobs"
      description="Search open roles by keyword, location and experience."
    >
      <FilterBar />
    </PagePlaceholder>
  )
}

export default SearchJobsPage
