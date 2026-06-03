import { WebTracerProvider, BatchSpanProcessor } from "@opentelemetry/sdk-trace-web";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { ZoneContextManager } from "@opentelemetry/context-zone";
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";
import { onLCP, onCLS, onINP } from "web-vitals";
import { trace } from "@opentelemetry/api";

const COLLECTOR = import.meta.env.VITE_OTEL_COLLECTOR_URL ?? "http://localhost:4320";
const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:9100";

const provider = new WebTracerProvider({
  resource: resourceFromAttributes({ [ATTR_SERVICE_NAME]: "checkout-frontend" }),
  spanProcessors: [
    new BatchSpanProcessor(new OTLPTraceExporter({ url: `${COLLECTOR}/v1/traces` })),
  ],
});

provider.register({ contextManager: new ZoneContextManager() });

registerInstrumentations({
  instrumentations: [
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: [new RegExp(API_BASE)],
    }),
  ],
});

const tracer = trace.getTracer("web-vitals");
function reportVital(name: string, value: number) {
  const span = tracer.startSpan(`web-vital.${name}`);
  span.setAttribute("web_vital.value", value);
  span.end();
}
onLCP((m) => reportVital("LCP", m.value));
onCLS((m) => reportVital("CLS", m.value));
onINP((m) => reportVital("INP", m.value));
