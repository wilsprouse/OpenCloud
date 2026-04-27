/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    typescript: {
      ignoreBuildErrors: true,
    },
    images: {
      unoptimized: true,
    },
    env: {
      REACT_APP_BACKEND: process.env.REACT_APP_BACKEND,
    },
    async rewrites() {
      // Use environment variable for backend URL, fallback to localhost:3030
      const backendUrl = process.env.BACKEND_URL || 'http://localhost:3030';
      return {
        // beforeFiles rewrites run before page/static file resolution.
        // More-specific patterns are listed first so Next.js matches them first.
        beforeFiles: [
          // Direct API path: /api/<endpoint> → Go backend.
          // This is the primary path used in production (nginx routes /api/ directly to
          // the Go backend, so this rule mainly covers the dev-server / localhost:3000 path).
          {
            source: '/api/:path*',
            destination: `${backendUrl}/:path*`,
          },
          // Gateway-prefixed API path: /<prefix>/api/<endpoint> → Go backend.
          // Handles the case where OpenCloud is accessed through a gateway sub-path
          // (e.g. curl -u "<token>:" https://domain.com/myprefix/api/get-containers).
          // Note: `:prefix` matches a single path segment, so /api/:path* (handled above)
          // is never captured here — the remaining literal "/api/" would need to appear
          // after the first segment, which doesn't happen for bare /api/... paths.
          {
            source: '/:prefix/api/:path*',
            destination: `${backendUrl}/:path*`,
          },
          // Gateway-prefixed direct path: /<prefix>/<endpoint> → Go backend,
          // but ONLY when an Authorization header is present (CLI token auth).
          // This allows: curl -u "<token>:" https://domain.com/myprefix/get-containers
          // Browser UI requests never carry an Authorization header (they use cookies),
          // so normal page navigation through the same prefix is unaffected.
          //
          // Ordering note: this rule is listed AFTER /:prefix/api/:path* so that
          // requests like /:prefix/api/:endpoint with an Authorization header still
          // match the more-specific api rule above (Next.js stops at the first match).
          {
            source: '/:prefix/:path*',
            has: [{ type: 'header', key: 'authorization' }],
            destination: `${backendUrl}/:path*`,
          },
        ],
        afterFiles: [],
        fallback: [],
      }
    },
  }
  
  export default nextConfig