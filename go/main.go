package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"encoding/json"
	"log/slog"

	"cloud.google.com/go/vertexai/genai"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var model *genai.GenerativeModel
var counter metric.Int64Counter
var MetricLabels []attribute.KeyValue = []attribute.KeyValue{attribute.Key("language").String("go")}

const scopeName = "o11y/demo/go"

func main() {
	ctx := context.Background()
	projectID, err := ProjectID(ctx)
	if err != nil {
		return
	}
	region, err := Region(ctx)
	if err != nil {
		return
	}

	setupLogging()
	shutdown, err := setupTelemetry(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "error setting up OpenTelemetry", slog.Any("error", err))
		os.Exit(1)
	}
	meter := otel.Meter(scopeName)
	counter, err = meter.Int64Counter("model_call_counter")
	if err != nil {
		slog.ErrorContext(ctx, "error setting up OpenTelemetry", slog.Any("error", err))
		os.Exit(1)
	}

	var client *genai.Client
	client, err = genai.NewClient(ctx, projectID, region)
	if err != nil {
		slog.ErrorContext(ctx, "Failed initialize GenAI client", slog.Any("error", err))
		os.Exit(1)
	}
	defer client.Close()
	model = client.GenerativeModel("gemini-1.5-flash-001")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.Handle("/facts", wireHttpHandler("/facts", factsHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err = errors.Join(http.ListenAndServe(":"+port, mux), shutdown(ctx)); err != nil {
		slog.ErrorContext(ctx, "Failed to start the server", slog.Any("error", err))
		os.Exit(1)
	}
}

func factsHandler(w http.ResponseWriter, r *http.Request) {
	animal := r.URL.Query().Get("animal")
	if animal == "" {
		animal = "dog"
	}

	prompt := fmt.Sprintf("Give me 10 fun facts about %s. Convert result to HTML format without markdown backticks.", animal)
	resp, err := model.GenerateContent(r.Context(), genai.Text(prompt))
	if err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to marshal response to JSON", slog.Any("error", err))
	} else {
		slog.DebugContext(r.Context(), "content is generated", slog.String("animal", animal),
			slog.String("prompt", prompt), slog.String("response", string(jsonBytes)))
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		counter.Add(r.Context(), 1, metric.WithAttributes(MetricLabels...))
		htmlContent := resp.Candidates[0].Content.Parts[0]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContent)
	}
}
