const GATEWAY_TENANT_PATTERN = /^[a-z0-9._-]{1,95}$/;

export function readGatewayTenant(value: string | undefined): string | undefined {
  return value !== undefined && GATEWAY_TENANT_PATTERN.test(value) ? value : undefined;
}

export function gatewayOriginConfigured(value: string | undefined): boolean {
  return value !== undefined && value.length > 0;
}
