/* eslint-disable no-console */
import { diag, DiagConsoleLogger, DiagLogLevel } from '@opentelemetry/api';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-proto';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { resourceFromAttributes } from '@opentelemetry/resources';
import {
  PeriodicExportingMetricReader,
  ConsoleMetricExporter,
} from '@opentelemetry/sdk-metrics';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { SimpleSpanProcessor } from '@opentelemetry/sdk-trace-node';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION, ATTR_SERVICE_INSTANCE_ID } from '@opentelemetry/semantic-conventions';

diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);

const traceExporter = new OTLPTraceExporter();

export const sdk = new NodeSDK({
  resource: resourceFromAttributes({
    [ATTR_SERVICE_NAME]: 'blockscout_frontend',
    [ATTR_SERVICE_VERSION]: process.env.NEXT_PUBLIC_GIT_TAG || process.env.NEXT_PUBLIC_GIT_COMMIT_SHA || 'unknown_version',
    [ATTR_SERVICE_INSTANCE_ID]:
        process.env.NEXT_PUBLIC_APP_INSTANCE ||
        process.env.NEXT_PUBLIC_APP_HOST?.replace('.blockscout.com', '').replaceAll('-', '_') ||
        'unknown_app',
  }),
  spanProcessors: [ new SimpleSpanProcessor(traceExporter) ],
  metricReaders: [ new PeriodicExportingMetricReader({
    exporter:
      process.env.NODE_ENV === 'production' ?
        new OTLPMetricExporter() :
        new ConsoleMetricExporter(),
  }) ],
  instrumentations: [
    getNodeAutoInstrumentations({
      '@opentelemetry/instrumentation-http': {
        ignoreIncomingRequestHook: (request) => {
          try {
            if (!request.url) {
              return false;
            }
            const url = new URL(request.url, `http://${ request.headers.host }`);
            if (
              url.pathname.startsWith('/_next/static/') ||
              url.pathname.startsWith('/_next/data/') ||
              url.pathname.startsWith('/explorer/assets/') ||
              url.pathname.startsWith('/explorer/static/')
            ) {
              return true;
            }
          } catch (error) {}
          return false;
        },
      },
    }),
  ],
});

if (process.env.OTEL_SDK_ENABLED === 'true') {
  sdk.start();

  process.on('SIGTERM', () => {
    sdk
      .shutdown()
      .then(() => console.log('Tracing terminated'))
      .catch((error) => console.log('Error terminating tracing', error))
      .finally(() => process.exit(0));
  });
}
