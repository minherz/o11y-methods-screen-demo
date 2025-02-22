import os

import json, logging
from opentelemetry import metrics, trace
from opentelemetry.exporter.cloud_monitoring import CloudMonitoringMetricsExporter
from opentelemetry.exporter.cloud_trace import CloudTraceSpanExporter
from opentelemetry.resourcedetector.gcp_resource_detector import GoogleCloudResourceDetector
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import get_aggregated_resources, Resource, CLOUD_ACCOUNT_ID, SERVICE_NAME
from opentelemetry.sdk.trace.export import BatchSpanProcessor
import sys

class JsonFormatter(logging.Formatter):
    def format(self, record):
        span = trace.get_current_span()
        json_log_object = {
            'severity': record.levelname,
            'message': record.getMessage(),
            "logging.googleapis.com/trace": google_trace_id_format(span.get_span_context().trace_id),
            "logging.googleapis.com/spanId": trace.format_span_id(span.get_span_context().span_id),            
        }
        json_log_object.update(getattr(record, 'json_fields', {}))
        return json.dumps(json_log_object)

logger = logging.getLogger(__name__)
sh = logging.StreamHandler(sys.stdout)
sh.setFormatter(JsonFormatter())
logger.addHandler(sh)
logger.setLevel(logging.DEBUG)

resource = get_aggregated_resources(
    [GoogleCloudResourceDetector(raise_on_error=False)]
)
resource = resource.merge(Resource.create(attributes={
    SERVICE_NAME: os.getenv("K_SERVICE"),
}))

meter_provider = MeterProvider(
    resource=resource,
    metric_readers=[
        PeriodicExportingMetricReader(
            CloudMonitoringMetricsExporter(), export_interval_millis=5000
        )
    ],
)
metrics.set_meter_provider(meter_provider)
meter = metrics.get_meter(__name__)

trace_provider = TracerProvider(resource=resource)
processor = BatchSpanProcessor(CloudTraceSpanExporter(
    # send all resource attributes
    resource_regex=r".*"
))
trace_provider.add_span_processor(processor)
trace.set_tracer_provider(trace_provider)

def google_trace_id_format(trace_id: int) -> str:
    project_id = resource.attributes[CLOUD_ACCOUNT_ID]
    return f'projects/{project_id}/traces/{trace.format_trace_id(trace_id)}'