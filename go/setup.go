package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"

	cloudmetric "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric"
	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"

	"cloud.google.com/go/compute/metadata"
)

var (
	projectID string
)

func ProjectID(ctx context.Context) (string, error) {
	pid := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if pid == "" {
		return metadata.ProjectIDWithContext(ctx)
	}
	return pid, nil
}

func Region(ctx context.Context) (string, error) {
	region, err := metadata.GetWithContext(ctx, "instance/region")
	if err != nil {
		return "", err
	}
	// parse region from fully qualified name projects/<projNum>/regions/<region>
	if pos := strings.LastIndex(region, "/"); pos >= 0 {
		region = region[pos+1:]
	}
	return region, nil
}

func setupLogging() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(group []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.LevelKey:
				a.Key = "severity"
				if level := a.Value.Any().(slog.Level); level == slog.LevelWarn {
					a.Value = slog.StringValue("WARNING")
				}
			case slog.MessageKey:
				a.Key = "message"
			case slog.TimeKey:
				a.Key = "timestamp"
			}
			return a
		},
	}
	jsonHandler := slog.NewJSONHandler(os.Stdout, opts)
	instrumentedHandler := handlerWithSpanContext(jsonHandler)
	slog.SetDefault(slog.New(instrumentedHandler))
}

type spanContextLogHandler struct {
	slog.Handler
}

func handlerWithSpanContext(handler slog.Handler) *spanContextLogHandler {
	return &spanContextLogHandler{Handler: handler}
}

func (t *spanContextLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if s := trace.SpanContextFromContext(ctx); s.IsValid() {
		trace := fmt.Sprintf("projects/%s/traces/%s", projectID, s.TraceID())
		record.AddAttrs(
			slog.Any("logging.googleapis.com/trace", trace),
		)
		record.AddAttrs(
			slog.Any("logging.googleapis.com/spanId", s.SpanID()),
		)
		record.AddAttrs(
			slog.Bool("logging.googleapis.com/trace_sampled", s.TraceFlags().IsSampled()),
		)
	}
	return t.Handler.Handle(ctx, record)
}

func setupTelemetry(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	projectID, err = ProjectID(ctx)
	if err != nil {
		err = errors.Join(err, shutdown(ctx))
		return
	}
	res, err2 := resource.New(
		ctx,
		resource.WithDetectors(gcp.NewDetector()),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(semconv.ServiceNameKey.String(os.Getenv("K_SERVICE"))),
	)
	if err2 != nil {
		err = errors.Join(err2, shutdown(ctx))
		return
	}

	otel.SetTextMapPropagator(autoprop.NewTextMapPropagator())

	texporter, err2 := cloudtrace.New(cloudtrace.WithProjectID(projectID))
	if err2 != nil {
		err = errors.Join(err2, shutdown(ctx))
		return
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(texporter))
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	mexporter, err2 := cloudmetric.New(cloudmetric.WithProjectID(projectID))
	if err2 != nil {
		err = errors.Join(err2, shutdown(ctx))
		return
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexporter)),
		sdkmetric.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	return shutdown, nil
}

func wireHttpHandler(route string, handleFn http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(otelhttp.WithRouteTag(route, handleFn), route)
}
