/** @type {import('next').NextConfig} */
const nextConfig = {
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
  }
  
  export default nextConfig