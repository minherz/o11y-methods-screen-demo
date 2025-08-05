package com.example.demo;

import com.google.auth.oauth2.GoogleCredentials;

import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.exporter.otlp.trace.OtlpGrpcSpanExporter;
import io.opentelemetry.sdk.autoconfigure.spi.AutoConfigurationCustomizer;
import io.opentelemetry.sdk.autoconfigure.spi.AutoConfigurationCustomizerProvider;
import io.opentelemetry.sdk.autoconfigure.spi.ConfigProperties;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.sdk.trace.export.BatchSpanProcessor;
import io.opentelemetry.sdk.trace.samplers.Sampler;
import io.opentelemetry.sdk.trace.SdkTracerProviderBuilder;
import io.opentelemetry.semconv.ServiceAttributes;

import org.springframework.context.annotation.Configuration;

import java.io.IOException;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

@Configuration
public class OpenTelemetryCustomizedConfigurationProvider
    implements AutoConfigurationCustomizerProvider {

  @Override
  public void customize(AutoConfigurationCustomizer autoConfiguration) {
    autoConfiguration
        .addResourceCustomizer(this::setupGoogleProject)
        .addTracerProviderCustomizer(this::configureSdkTracerProvider);
  }

  private SdkTracerProviderBuilder configureSdkTracerProvider(
      SdkTracerProviderBuilder tracerProvider, ConfigProperties config) {
    GoogleCredentials credentials;
    try {
      credentials = GoogleCredentials.getApplicationDefault();
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
    var otlpGrpcSpanExporter = OtlpGrpcSpanExporter.builder()
        .setHeaders(
            () -> {
              Map<String, List<String>> gcpHeaders;
              try {
                credentials.refreshIfExpired();
                gcpHeaders = credentials.getRequestMetadata();
              } catch (IOException e) {
                // Handle authentication error
                throw new RuntimeException(e);
              }
              Map<String, String> flattenedHeaders = gcpHeaders.entrySet().stream()
                  .collect(
                      Collectors.toMap(
                          Map.Entry::getKey,
                          entry -> entry.getValue().stream()
                              .filter(Objects::nonNull)
                              .filter(s -> !s.isEmpty())
                              .collect(Collectors.joining(",")),
                          (v1, v2) -> v2));
              return flattenedHeaders;
            })
        .setTimeout(2, TimeUnit.SECONDS)
        // NOTE: endpoint URI has to be in HTTP format
        .setEndpoint("https://telemetry.googleapis.com")
        .build();
    return tracerProvider
        .setSampler(Sampler.alwaysOn())
        .addSpanProcessor(
            BatchSpanProcessor.builder(otlpGrpcSpanExporter)
                .setScheduleDelay(100, TimeUnit.MILLISECONDS)
                .build());
  }

  private Resource setupGoogleProject(Resource originalResource, ConfigProperties config) {
    var serviceName = System.getenv().getOrDefault("K_SERVICE", "java-o11y-demo-app");
    var resource = Resource.create(
        Attributes.builder()
            .put("gcp.project_id", Metadata.projectId())
            .put(ServiceAttributes.SERVICE_NAME, serviceName)
            .build());
    return originalResource.merge(resource);
  }
}
