import { useEffect } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import Footer from './Footer'
import Navbar from './Navbar'

function Layout() {
  const { pathname, hash } = useLocation()

  // React Router doesn't scroll on navigation by itself: jump to the #section
  // when there is one, otherwise back to the top of the new page.
  useEffect(() => {
    if (hash) {
      document.getElementById(hash.slice(1))?.scrollIntoView()
    } else {
      window.scrollTo(0, 0)
    }
  }, [pathname, hash])

  return (
    <div className="app">
      <Navbar />
      <main className="app__main">
        <Outlet />
      </main>
      <Footer />
    </div>
  )
}

export default Layout
