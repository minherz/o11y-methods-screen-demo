import opentelemetry from "@opentelemetry/api";
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';
import { MeterProvider, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { AlwaysOnSampler, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { Resource } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import FastifyOtelInstrumentation from '@fastify/otel';
import { HttpInstrumentation } from '@opentelemetry/instrumentation-http';
import { TraceExporter } from '@google-cloud/opentelemetry-cloud-trace-exporter';
import { MetricExporter } from '@google-cloud/opentelemetry-cloud-monitoring-exporter';
import { GcpDetectorSync } from '@google-cloud/opentelemetry-resource-util';
// import { diag, DiagConsoleLogger, DiagLogLevel } from "@opentelemetry/api";
// diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.DEBUG);

export const fastifyOtelInstrumentation = new FastifyOtelInstrumentation({ servername: process.env.K_SERVICE });
export const setupTelemetry = function () {
    const gcpResource = new Resource({
        [ATTR_SERVICE_NAME]: process.env.K_SERVICE,
    }).merge(new GcpDetectorSync().detect())

    const tracerProvider = new NodeTracerProvider({
        resource: gcpResource,
        sampler: new AlwaysOnSampler(),
        spanProcessors: [new SimpleSpanProcessor(new TraceExporter({
            resourceFilter: /^service\./
        }))],
    });
    registerInstrumentations({
        tracerProvider: tracerProvider,
        instrumentations: [
            new HttpInstrumentation(),
        ],
    });
    fastifyOtelInstrumentation.setTracerProvider(tracerProvider);
    // Initialize the OpenTelemetry APIs to use the NodeTracerProvider bindings
    tracerProvider.register();

    const meterProvider = new MeterProvider({
        resource: gcpResource,
        readers: [new PeriodicExportingMetricReader({
            // Export metrics every second (default quota is 30,000 time series ingestion requests per minute)
            exportIntervalMillis: 1_000,
            exporter: new MetricExporter(),
        })],
    });
    opentelemetry.metrics.setGlobalMeterProvider(meterProvider);
}
