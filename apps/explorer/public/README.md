**Directories**
- `/icons` - Folder for SVG-sprite assets, generated at build time.
- `/static` - Folder for static assets that are consistent between app re-runs but may differ from one build version to another.
- `/assets` - Folder for dynamically generated assets during the app start, such as the favicon bundle, ENV variables file, and external app configurations.

**Files**
- `openapi.yaml` - OpenAPI document of the explorer backend shown on the REST API tab of the API documentation page. It is the output of `services/explorer/deploy/tools/gen-swagger.py` and is regenerated together with the backend routes.
