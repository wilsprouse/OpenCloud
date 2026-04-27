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
        ],
        afterFiles: [],
        fallback: [],
      }
    },
  }
  
  export default nextConfig