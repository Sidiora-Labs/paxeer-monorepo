# Changelog

Model compatibility notes for `chainflow-data-gateway`.

## Unreleased

- Response-only models are marked `#[non_exhaustive]`, so new response
  fields can be added in future versions without a breaking release;
  request models stay literally constructible.
