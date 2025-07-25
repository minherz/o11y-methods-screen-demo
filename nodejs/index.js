import { VertexAI } from '@google-cloud/vertexai';
import { GoogleAuth } from 'google-auth-library';

let generativeModel, traceIdPrefix;
const auth = new GoogleAuth();
auth.getProjectId().then(result => {
    const vertex = new VertexAI({ project: result });
    generativeModel = vertex.getGenerativeModel({
        model: process.env.MODEL_NAME || 'gemini-2.5-flash'
    });
    traceIdPrefix = `projects/${result}/traces/`;
});

// setup tracing and monitoring OTel providers
import { setupTelemetry, fastifyOtelInstrumentation } from './setup.js';
setupTelemetry();

import { trace, context, metrics } from "@opentelemetry/api";
function getCurrentSpan() {
    const current_span = trace.getSpan(context.active());
    return {
        trace_id: current_span.spanContext().traceId,
        span_id: current_span.spanContext().spanId,
        flags: current_span.spanContext().traceFlags
    };
};

const meter = metrics.getMeter("o11y/demo/nodejs");
const counter = meter.createCounter("model_call_counter");

import path from 'path';
import { fileURLToPath } from 'url';
import Fastify from 'fastify'
import fastifyStatic from '@fastify/static';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const fastify = Fastify({});
fastify.register(fastifyStatic, {
    root: path.join(__dirname, 'static')
});
await fastify.register(fastifyOtelInstrumentation.plugin());

fastify.get('/', function (req, reply) {
    reply.sendFile('index.html')
});

fastify.get('/facts', async function (request, reply) {
    try {
        const subject = (request.query.subject || request.query.animal) || 'dog';
        const prompt = `Give me 10 fun facts about ${subject}. Return this as html without backticks.`
        const resp = await generativeModel.generateContent(prompt);
        const span = getCurrentSpan();
        console.log(JSON.stringify({
            'severity': 'DEBUG',
            'message': 'Content is generated',
            'subject': subject,
            'prompt': prompt,
            'response': resp.response,
            "logging.googleapis.com/trace": traceIdPrefix + span.trace_id,
            "logging.googleapis.com/spanId": span.span_id,
        }));
        counter.add(1, { language: 'nodejs' });
        const html = resp.response.candidates[0].content.parts[0].text;
        reply.type('text/html').send(html);
    }
    catch (error) {
        reply.type('text/html').send(error);
    }
})

const PORT = parseInt(process.env.PORT || '8080');
fastify.listen({ host: '0.0.0.0', port: PORT }, function (err, address) {
    if (err) {
        console.error(err);
        process.exit(1);
    }
    console.log(`codelab-genai: listening on ${address}`);
})