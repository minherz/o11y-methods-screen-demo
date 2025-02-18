# Observability Methods Screen Demo

This is a set of demo applications in four programming languages.
The applications demonstrate observability data ingestion to Google Cloud.
The shown methods follow best practices and recommendations of writing logs, traces and metrics to Google Cloud.

The applications are deployed to Cloud Run.

The applications are open for public access and exposed to Deny of Wallet attack that generates high bills to your cloud account.

The applications demonstrate the following techniques and practices for Go, Java, NodeJS and Python:

* Use of embedded logging agents ([documentation](https://cloud.google.com/run/docs/logging#container-logs))
* Writing structured logs ([documentation](https://cloud.google.com/logging/docs/structured-logging#special-payload-fields))
* Use of OpenTelemetry SDK
* Correlating logs, traces and metrics
* Adding metrics based on written logs (see below)

## Deployment

The following steps describe deployment of the Java demo application as a Cloud Run service to the Google Cloud project with ID `PROJECT_ID`.
If you want to deploy the application in another language, you that language folder name instead of `./java`.

1. Open a terminal window.
1. Change current directory to be the root directory of this repository.
1. Run the following command to enable all necessary APIs:

   ```shell
   gcloud services enable aiplatform.googleapis.com artifactregistry.googleapis.com \
     cloudbuild.googleapis.com run.googleapis.com \
     logging.googleapis.com monitoring.googleapis.com cloudtrace.googleapis.com \
     --project=PROJECT_ID
   ```

1. Run the following command to deploy the demo app:

   ```shell
   gcloud run deploy codelab-o11y-service \
     --source="./java/" \
     --region=us-central1 \
     --allow-unauthenticated
   ```

> [!TIP]
> You can deploy to another region by replacing "us-central1" in the last command with your region name.

## Cleanup

The fastest and complete way to cleanup is to delete the Google Cloud project where you deployed the demo application(s). For a project with ID `PROJECT_ID` run the following gcloud CLI command:

```shell
gcloud projects delete PROJECT_ID --quiet
```

Deleting just Cloud Run services does not clean up artifacts created by Cloud Build and does not delete observability data in Cloud Logging, Cloud Monitoring and Cloud Trace.

## Create log-based metrics

Code writes custom metrics using OpenTelemetry SDK. It is possible to use [log-based metrics](https://cloud.google.com/logging/docs/logs-based-metrics) using the logs that demo application writes. The following commands will create a log-based metric for an application that writes logs into the project with ID `PROJECT_ID`. Because the demo applications in all languages write the same data to Cloud Logging the command will calculate it for all apps while using the name of the Cloud Run service as a label (similar to the label set for the custom metrics).

```shell
cat > "/tmp/log-metric-config.json" <<EOF
{
  "name": "lb_model_call_count",
  "description": "Number of log entries capturing successful call to model inference",
  "filter": "LOG_ID(\"run.googleapis.com%2Fstdout\") AND severity=DEBUG",
  "labelExtractors": {
    "service": "resource.service_name"
  }
}
EOF

gcloud logging metrics create lb_model_call_count \
  --project=PROJECT_ID \
  --config-from-file=/tmp/log-metric-config.json
```
