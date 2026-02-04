/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    eslint: {
      ignoreDuringBuilds: true,
    },
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
      return [
        {
          source: '/api/:path*',
          destination: `${backendUrl}/:path*`,
        },
      ]
    },
  }
  
  export default nextConfig