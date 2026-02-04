/*
TODO: This will eventually be used when we get into Auth
*/
import axios from "axios";
import { useRouter } from 'next/navigation';

        /*const res = await axios.get("http://localhost:3030/get-server-metrics", {
          headers: {
             "Content-Type": "application/json",
          },
        });*/

const client = axios.create({
  baseURL: process.env.REACT_APP_BACKEND || "/api",
  headers: {
    "Content-Type": "application/json",
    //'Access-Control-Allow-Origin' : '*',
    //'Access-Control-Allow-Methods':'GET,PUT,POST,DELETE,PATCH,OPTIONS',   
  }
});

// Setup interceptors for expired AccessTokens
/*client.interceptors.response.use(null, (error) => {
  if(error.config && error.response && error.response.status === 401) {
    return client.get("/user/get-auth/").then((resp) => {

      localStorage.setItem("access_token", resp.data.new_access_token);
      error.config.headers['AccessToken'] = resp.data.new_access_token;
      return axios.request(error.config);

    }).catch((error: any) => {

      localStorage.removeItem("access_token");
      localStorage.removeItem("refresh_token");
      localStorage.removeItem("username");
   
      const router = useRouter();

      router.push("/login");
    });
  }
  return Promise.reject(error);
});

const getErrorMessage = (error: any) => {
  let resp = "";

  try {
    resp = error.response.data.message;
  }
  catch(error) {
    resp = "Sorry, something went wrong. Please try again later.";
  }

  return resp;
};*/

export default client;
//export { getErrorMessage };
