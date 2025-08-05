import opentelemetry from "@opentelemetry/api";
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';
import { MeterProvider, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { AlwaysOnSampler, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { Resource } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import FastifyOtelInstrumentation from '@fastify/otel';
import { MetricExporter } from '@google-cloud/opentelemetry-cloud-monitoring-exporter';

import { GoogleAuth } from 'google-auth-library';
import grpc from '@grpc/grpc-js';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';

import { diag, DiagConsoleLogger, DiagLogLevel } from "@opentelemetry/api";
diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.DEBUG);

const serviceName = process.env.K_SERVICE || 'nodejs-otel-example'
const fastifyOtelInstrumentation = new FastifyOtelInstrumentation({
    servername: serviceName,
    registerOnInitialization: true,
});
export const setupTelemetry = async function (projectId) {
    const auth = new GoogleAuth({
        scopes: [
            'https://www.googleapis.com/auth/cloud-platform',
            'https://www.googleapis.com/auth/trace.append'
        ]
    });
    const authenticatedClient = await auth.getClient();
    const accessToken = await authenticatedClient.getAccessToken();
    const metadata = new grpc.Metadata();
    metadata.set('Authorization', `Bearer ${accessToken.token}`);
    const tracerProvider = new NodeTracerProvider({
        resource: new Resource({
            [ATTR_SERVICE_NAME]: serviceName,
            'gcp.project_id': projectId,
        }),
        sampler: new AlwaysOnSampler(),
        spanProcessors: [new SimpleSpanProcessor(new OTLPTraceExporter({
            url: 'https://telemetry.googleapis.com', // Your OTLP endpoint
            metadata: metadata,
            credentials: grpc.credentials.combineChannelCredentials(
                grpc.credentials.createSsl(),
                grpc.credentials.createFromGoogleCredential(authenticatedClient)
            ),
        }))],
    });
    fastifyOtelInstrumentation.setTracerProvider(tracerProvider);
    tracerProvider.register();

    const meterProvider = new MeterProvider({
        resource: new Resource({
            [ATTR_SERVICE_NAME]: 'trace-test-service',
        }),
        readers: [new PeriodicExportingMetricReader({
            // Export metrics every second (default quota is 30,000 time series ingestion requests per minute)
            exportIntervalMillis: 1_000,
            exporter: new MetricExporter(),
        })],
    });
    opentelemetry.metrics.setGlobalMeterProvider(meterProvider);
}
