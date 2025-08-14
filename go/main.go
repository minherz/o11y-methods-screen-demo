package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"encoding/json"
	"log/slog"

	"google.golang.org/genai"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var modelName string
var client *genai.Client
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

	client, err = genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: region,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed initialize GenAI client", slog.Any("error", err))
		os.Exit(1)
	}
	modelName = os.Getenv("MODEL_NAME")
	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}
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

func getSubject(r *http.Request) string {
	v := r.URL.Query().Get("subject")
	if v != "" {
		return v
	}
	// for backward compatability check 'animal' query attribute
	v = r.URL.Query().Get("animal")
	if v != "" {
		return v
	}
	return "dog"
}

func factsHandler(w http.ResponseWriter, r *http.Request) {
	subject := getSubject(r)
	prompt := fmt.Sprintf("Give me 10 fun facts about %s. Convert result to HTML format without markdown backticks.", subject)
	resp, err := client.Models.GenerateContent(r.Context(), modelName, genai.Text(prompt), nil)
	if err != nil {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, err.Error())
		return
	}
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to marshal response to JSON", slog.Any("error", err))
	} else {
		slog.DebugContext(r.Context(), "content is generated", slog.String("subject", subject),
			slog.String("prompt", prompt), slog.String("response", string(jsonBytes)))
	}
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		counter.Add(r.Context(), 1, metric.WithAttributes(MetricLabels...))
		htmlContent := resp.Candidates[0].Content.Parts[0].Text
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContent)
	}
}
