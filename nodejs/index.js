import Fastify from 'fastify'
import fastifyStatic from '@fastify/static';
import { GoogleGenAI } from '@google/genai';
import { GoogleAuth } from 'google-auth-library';
import { setupTelemetry } from './setup.js';
import { context, metrics, trace } from '@opentelemetry/api';
import path from 'path';
import { fileURLToPath } from 'url';

const fastify = Fastify({});
const auth = new GoogleAuth();
const projectId = await auth.getProjectId();

const genAI = new GoogleGenAI({
    vertexai: true,
    project: projectId,
    location: 'us-central1',
});
const traceIdPrefix = `projects/${projectId}/traces/`;
await setupTelemetry(projectId);

const meter = metrics.getMeter("o11y/demo/nodejs");
const counter = meter.createCounter("model_call_counter");

function getCurrentSpan() {
    const current_span = trace.getSpan(context.active());
    if (current_span) {
        const ctx = current_span.spanContext();
        if (ctx) {
            return {
                trace_id: ctx.traceId,
                span_id: ctx.spanId,
                flags: ctx.traceFlags
            };
        }
    }
    return {
        trace_id: "undefined",
        span_id: "undefined",
        flags: "0"
    };
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
fastify.register(fastifyStatic, {
    root: path.join(__dirname, 'static')
});

fastify.get('/', function (req, reply) {
    reply.sendFile('index.html')
});

fastify.get('/facts', async function (request, reply) {
    try {
        const subject = (request.query.subject || request.query.animal) || 'dog';
        const prompt = `Give me 10 fun facts about ${subject}. Return this as html without backticks.`
        const resp = await genAI.models.generateContent({
            model: process.env.MODEL_NAME || 'gemini-2.5-flash',
            contents: prompt,
        });
        const span = getCurrentSpan();
        console.log(JSON.stringify({
            'severity': 'DEBUG',
            'message': 'Content is generated',
            'subject': subject,
            'prompt': prompt,
            'response': resp,
            "logging.googleapis.com/trace": traceIdPrefix + span.trace_id,
            "logging.googleapis.com/spanId": span.span_id,
        }));
        counter.add(1, { language: 'nodejs' });
        const html = resp.text;
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
    console.log(`server is listening on ${address}`);
})