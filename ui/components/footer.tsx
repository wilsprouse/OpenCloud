/**
 * Footer component displayed at the bottom of every page (except login).
 * Provides links to documentation, the OpenCloud GitHub repository, and the project license.
 */
export function Footer() {
  return (
    <footer className="w-full border-t bg-background/95">
      <div className="container flex h-12 items-center justify-center gap-6 text-sm text-muted-foreground">
        <a
          href="https://github.com/WavexSoftware/OpenCloud/wiki"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-foreground transition-colors"
        >
          Documentation
        </a>
        <a
          href="https://github.com/WavexSoftware/OpenCloud"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-foreground transition-colors"
        >
          Repository
        </a>
        <a
          href="https://github.com/WavexSoftware/OpenCloud?tab=GPL-3.0-1-ov-file"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-foreground transition-colors"
        >
          License
        </a>
      </div>
    </footer>
  )
}
