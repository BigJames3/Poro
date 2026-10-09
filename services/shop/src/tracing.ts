import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ExpressInstrumentation } from '@opentelemetry/instrumentation-express';
import { HttpInstrumentation } from '@opentelemetry/instrumentation-http';
import { PgInstrumentation } from '@opentelemetry/instrumentation-pg';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { NodeSDK } from '@opentelemetry/sdk-node';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';

import { SERVICE_NAME, SERVICE_VERSION } from './version';

// Must be imported before express and pg so their instrumentation can patch them.
// OTEL_EXPORTER_OTLP_ENDPOINT is the OTLP/HTTP base URL, e.g. http://otel-collector:4318.
const endpoint = process.env.OTEL_EXPORTER_OTLP_ENDPOINT?.trim();

export const tracing =
  endpoint === undefined || endpoint === ''
    ? undefined
    : new NodeSDK({
        resource: resourceFromAttributes({
          [ATTR_SERVICE_NAME]: SERVICE_NAME,
          [ATTR_SERVICE_VERSION]: SERVICE_VERSION,
        }),
        traceExporter: new OTLPTraceExporter({ url: `${endpoint.replace(/\/$/, '')}/v1/traces` }),
        instrumentations: [
          new HttpInstrumentation({
            ignoreIncomingRequestHook: (req) =>
              (req.url ?? '').startsWith('/health') || req.url === '/metrics',
          }),
          new ExpressInstrumentation(),
          new PgInstrumentation(),
        ],
      });

tracing?.start();
