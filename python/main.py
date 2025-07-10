import os
from flask import Flask, render_template, request
from setup import logger
from metadata import resource_project, resource_region
from opentelemetry import metrics
from opentelemetry.instrumentation.requests import RequestsInstrumentor
from opentelemetry.instrumentation.flask import FlaskInstrumentor
import vertexai
from vertexai.generative_models import GenerativeModel

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
def index():
    return send_from_directory('static', 'favicon.ico', mimetype='image/vnd.microsoft.icon')

@app.route('/facts')
def fun_facts():
    vertexai.init(project=project, location=region)
    model = GenerativeModel(os.environ.get('MODEL_NAME', 'gemini-2.5-flash'))
    subject = request.args.get('subject', 'dog')
    prompt = f'Give me 10 fun facts about {subject}. Return this as html without backticks.'
    response = model.generate_content(prompt)
    json_fields = {
         'subject': subject,
         'prompt': prompt,
         'response': response.to_dict(),
    }
    logger.debug('content is generated', extra={'json_fields': json_fields})
    requests_counter.add(1, METRIC_LABELS)
    return response.text

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=int(os.environ.get('PORT', 8080)))