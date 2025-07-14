import os

from datetime import datetime, date
import json, logging
from metadata import resource_project
import google.auth
import grpc
from google.auth.transport.requests import Request
from google.auth.transport.grpc import AuthMetadataPlugin
from opentelemetry import metrics, trace
from opentelemetry.exporter.cloud_monitoring import CloudMonitoringMetricsExporter
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.resourcedetector.gcp_resource_detector import GoogleCloudResourceDetector
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.sdk.resources import get_aggregated_resources, Resource, SERVICE_NAME
from opentelemetry.sdk.trace.export import BatchSpanProcessor
import sys

def json_serial(obj):
    if isinstance(obj, (datetime, date)):
        return obj.isoformat()
    raise TypeError(f"Object of type {obj.__class__.__name__} is not JSON serializable")

class JsonFormatter(logging.Formatter):
    def format(self, record):
        span = trace.get_current_span()
        json_log_object = {
            'severity': record.levelname,
            'message': record.getMessage(),
            'logging.googleapis.com/trace': google_trace_id_format(span.get_span_context().trace_id),
            'logging.googleapis.com/spanId': trace.format_span_id(span.get_span_context().span_id),            
        }
        json_log_object.update(getattr(record, 'json_fields', {}))
        return json.dumps(json_log_object, default=json_serial)

logger = logging.getLogger(__name__)
sh = logging.StreamHandler(sys.stdout)
sh.setFormatter(JsonFormatter())
logger.addHandler(sh)
logger.setLevel(logging.DEBUG)

# Retrieve and store Google application-default credentials
credentials, _ = google.auth.default()
# AuthMeatadataPlugin inserts credentials into each request
# request is used to refresh credentials upon expiry
auth_metadata_plugin = AuthMetadataPlugin(
    credentials=credentials, request=Request()
)

resource = get_aggregated_resources(
    [GoogleCloudResourceDetector(raise_on_error=False)]
)
resource = resource.merge(Resource.create(attributes={
    SERVICE_NAME: os.getenv('K_SERVICE'),
    'gcp.project_id': resource_project(),
}))

meter_provider = MeterProvider(
    resource=resource,
    metric_readers=[
        PeriodicExportingMetricReader(
            CloudMonitoringMetricsExporter(),
            export_interval_millis=5000,
        )
    ],
)
metrics.set_meter_provider(meter_provider)
meter = metrics.get_meter('o11y/demo/python')

channel_creds = grpc.composite_channel_credentials(
    grpc.ssl_channel_credentials(),
    grpc.metadata_call_credentials(auth_metadata_plugin),
)
trace_exporter = OTLPSpanExporter(
    credentials=channel_creds,
    endpoint='telemetry.googleapis.com:443',
)
trace_provider = TracerProvider(resource=resource)
trace_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
trace.set_tracer_provider(trace_provider)

def google_trace_id_format(trace_id: int) -> str:
    project_id = resource_project()
    return f'projects/{project_id}/traces/{trace.format_trace_id(trace_id)}'