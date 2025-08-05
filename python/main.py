import os
from flask import Flask, send_from_directory, request
from setup import logger
from metadata import resource_project, resource_region
from opentelemetry import metrics
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.instrumentation.flask import FlaskInstrumentor
from google import genai
from google.genai import errors

METRIC_LABELS = {'language': 'python'}

project = resource_project()
region = resource_region()
app = Flask(__name__)
FlaskInstrumentor().instrument_app(app)
RequestsInstrumentor().instrument()
meter = metrics.get_meter(__name__)
requests_counter = meter.create_counter(
    name="model_call_counter",
    description="number of model invocations",
    unit="1"
)

@app.route('/')
def index():
    return send_from_directory('static', 'index.html', mimetype='text/html')

@app.route('/favicon.ico')
def favicon():
    return send_from_directory('static', 'favicon.ico', mimetype='image/vnd.microsoft.icon')

@app.route('/facts')
def fun_facts():
    client = genai.Client(vertexai=True, project=project, location=region)
    model = os.environ.get('MODEL_NAME', 'gemini-2.5-flash')
    subject = request.args.get('subject', 'dog')
    prompt = f'Give me 10 fun facts about {subject}. Return this as html without backticks.'
    try:
        response = client.models.generate_content(model=model, contents=prompt)
    except errors.APIError as e:
        return e.message, e.code

    json_fields = {
         'subject': subject,
         'prompt': prompt,
         'response': response.model_dump(),
    }
    logger.debug('content is generated', extra={'json_fields': json_fields})
    requests_counter.add(1, METRIC_LABELS)
    return response.text

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=int(os.environ.get('PORT', 8080)))