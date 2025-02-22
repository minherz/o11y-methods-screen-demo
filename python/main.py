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
    return render_template('index.html') 

@app.route('/facts')
def fun_facts():
    project='genai-with-internet-001'
    region='us-central1'
    vertexai.init(project=project, location=region)
    model = GenerativeModel('gemini-1.5-flash')
    animal = request.args.get('animal', 'dog') 
    prompt = f'Give me 10 fun facts about {animal}. Return this as html without backticks.'
    response = model.generate_content(prompt)
    json_fields = {
         'animal': animal,
         'prompt': prompt,
         'response': response.to_dict(),
    }
    logger.debug('content is generated', extra={'json_fields': json_fields})
    requests_counter.add(1, METRIC_LABELS)
    return response.text

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=int(os.environ.get('PORT', 8080)))