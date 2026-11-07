export function humanServiceOrigin(): string {
  const service = process.env.LAYERX_HUMAN_SERVICE_URL;
  if (service === undefined) {
    throw new Error("LAYERX_HUMAN_SERVICE_URL must name the HTTPS human service");
  }
  const endpoint = new URL(service);
  if (
    endpoint.protocol !== "https:" ||
    endpoint.username !== "" ||
    endpoint.password !== "" ||
    !["/", "/human", "/human/"].includes(endpoint.pathname) ||
    endpoint.search !== "" ||
    endpoint.hash !== ""
  ) {
    throw new Error("LAYERX_HUMAN_SERVICE_URL must name the HTTPS human service");
  }
  return endpoint.origin;
}
