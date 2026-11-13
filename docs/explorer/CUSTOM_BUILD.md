# Building and running an explorer image

From `apps/explorer`, run `yarn build:docker`. The script builds `blockscout-frontend:local` using the monorepo root as its context and the [explorer Dockerfile](../../docker/explorer-frontend/Dockerfile).

Run the image with the environment configuration for the instance:

```sh
docker run -p 3000:3000 --env-file <path-to-env-file> blockscout-frontend:local
```

Open `http://localhost:3000/explorer`. The frontend's base path is `/explorer`.

The Dockerfile prepares configuration assets, the icon sprite and the Next.js standalone output. For a local production build, `yarn build:next` runs the frontend's preparation scripts before `next build`. See the [frontend scripts](../../apps/explorer/package.json) and [environment reference](./ENVS.md).
