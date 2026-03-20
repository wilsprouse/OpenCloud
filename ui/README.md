# Frontend
This repo contains a working, example frontend.

Please note that this frontend code is an example, and you can easily provide your own frontend and use it as you please. The backend is agnostic to the frontend that is calling it.

Eventually this will be pure js/css/html. Live with it. #NoBuild

## Container run presets

The Compute > Containers page can load optional image-specific runtime defaults from
`REACT_APP_CONTAINER_RUN_PRESETS`.

Example:

```json
[
  {
    "imagePatterns": ["minio"],
    "ports": [
      { "hostPort": "9000", "containerPort": "9000" },
      { "hostPort": "9001", "containerPort": "9001" }
    ],
    "envVars": [
      { "key": "MINIO_ROOT_USER", "value": "minioadmin" },
      { "key": "MINIO_ROOT_PASSWORD", "value": "minioadmin" }
    ],
    "volumes": [
      { "hostPath": "/data/minio", "containerPath": "/data" }
    ],
    "command": "server /data --address :9000 --console-address :9001"
  }
]
```

If the variable is unset or invalid, the page stays generic and no image-specific defaults are applied.
