const Footer = () => (
  <footer className="mt-10 flex items-center justify-center gap-1 pb-8 pt-4 text-[0.75rem] font-medium text-ink-subtle">
    <span>© {new Date().getFullYear()}</span>
    <a
      href="https://octelium.com"
      target="_blank"
      rel="noreferrer noopener"
      className="transition-colors duration-150 hover:text-ink-soft"
    >
      octelium.com
    </a>
  </footer>
);

export default Footer;
