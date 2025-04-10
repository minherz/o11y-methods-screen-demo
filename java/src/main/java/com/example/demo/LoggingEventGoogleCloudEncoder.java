package com.example.demo;

import static ch.qos.logback.core.CoreConstants.UTF_8_CHARSET;
import ch.qos.logback.core.encoder.EncoderBase;
import ch.qos.logback.classic.Level;
import ch.qos.logback.classic.spi.ILoggingEvent;

import com.google.cloud.ServiceOptions;
import com.google.gson.Gson;

import java.time.Instant;
import java.util.HashMap;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.SpanContext;
import io.opentelemetry.context.Context;


public class LoggingEventGoogleCloudEncoder extends EncoderBase<ILoggingEvent>  {
    private static final byte[] EMPTY_BYTES = new byte[0];
    private final Gson gson;
    private final String projectId;
    private final String tracePrefix;

    public LoggingEventGoogleCloudEncoder() {
        this.gson = new Gson();
        this.projectId = ServiceOptions.getDefaultProjectId();
        this.tracePrefix = "projects/" + (projectId == null ? "" : projectId) + "/traces/";
    }

    @Override
    public byte[] headerBytes() {
        return EMPTY_BYTES;
    }

    @Override
    public byte[] encode(ILoggingEvent e) {
        var timestamp = Instant.ofEpochMilli(e.getTimeStamp());
        var fields = new HashMap<String, Object>() {
            {
                put("timestamp", timestamp.toString());
                put("severity", severityFor(e.getLevel()));
                put("message", e.getMessage());
                SpanContext context = Span.fromContext(Context.current()).getSpanContext();
                if (context.isValid()) {
                    put("logging.googleapis.com/trace", tracePrefix + context.getTraceId());
                    put("logging.googleapis.com/spanId", context.getSpanId());
                    put("logging.googleapis.com/trace_sampled", Boolean.toString(context.isSampled()));
                }
            }
        };
        var params = e.getKeyValuePairs();
        if (params != null && params.size() > 0) {
            params.forEach(kv -> fields.putIfAbsent(kv.key, kv.value));
        }
        var data = gson.toJson(fields) + "\n";
        return data.getBytes(UTF_8_CHARSET);
    }

    @Override
    public byte[] footerBytes() {
        return EMPTY_BYTES;
    }

    private static String severityFor(Level level) {
        switch (level.toInt()) {
            case Level.TRACE_INT:
            return "DEBUG";
            case Level.DEBUG_INT:
            return "DEBUG";
            case Level.INFO_INT:
            return "INFO";
            case Level.WARN_INT:
            return "WARNING";
            case Level.ERROR_INT:
            return "ERROR";
            default:
            return "DEFAULT";
        }
    }
}