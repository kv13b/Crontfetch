import Logo from './Logo'

const year = new Date().getFullYear()

function Footer() {
  return (
    <footer className="footer">
      <div className="container footer__inner">
        <Logo />
        <ul className="footer__links">
          <li>
            <a href="#">Privacy</a>
          </li>
          <li>
            <a href="#">Terms</a>
          </li>
          <li>
            <a href="https://github.com/kv13b/Crontfetch" target="_blank" rel="noreferrer">
              GitHub
            </a>
          </li>
        </ul>
        <p className="footer__copy">© {year} CronFetch</p>
      </div>
    </footer>
  )
}

export default Footer
