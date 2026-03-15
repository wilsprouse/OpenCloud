import axios from "axios"

const client = axios.create({
  baseURL: process.env.REACT_APP_BACKEND || "/api",
  headers: {
    "Content-Type": "application/json",
  },
})

// Attach the access token to every outgoing request when available
client.interceptors.request.use((config) => {
  if (typeof window !== "undefined") {
    const token = localStorage.getItem("access_token")
    if (token) {
      config.headers["AccessToken"] = token
    }
  }
  return config
})

// On a 401 response, attempt a silent token refresh.
// If the refresh also fails, clear credentials and redirect to the login page.
// The login endpoint is excluded: a 401 there means bad credentials, not an
// expired session, and we must not trigger a redirect that would wipe the
// error message shown to the user.
client.interceptors.response.use(null, async (error) => {
  if (error.config && error.response && error.response.status === 401) {
    // Do not attempt a refresh for the login endpoint itself.
    if (error.config.url?.endsWith("/user/login")) {
      return Promise.reject(error)
    }

    try {
      const resp = await client.get("/user/get-auth/")
      const newToken: string | undefined = resp.data?.new_access_token

      if (!newToken) {
        throw new Error("Token refresh returned no access token")
      }

      const secure = window.location.protocol === "https:" ? "; Secure" : ""
      localStorage.setItem("access_token", newToken)
      document.cookie = `opencloud_session=${newToken}; path=/; SameSite=Strict${secure}`

      error.config.headers["AccessToken"] = newToken
      return axios.request(error.config)
    } catch {
      localStorage.removeItem("access_token")
      localStorage.removeItem("refresh_token")
      localStorage.removeItem("username")
      const secure = window.location.protocol === "https:" ? "; Secure" : ""
      document.cookie =
        `opencloud_session=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Strict${secure}`
      window.location.href = "/login"
    }
  }
  return Promise.reject(error)
})

export default client

