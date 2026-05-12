import { shortTraceId } from '../lib/format';

export function TraceIdChip({ traceId }: { traceId: string }) {
  if (!traceId) return null;
  const jaeger = import.meta.env.VITE_JAEGER_URL ?? 'http://localhost:16686';
  const copy = () => {
    void navigator.clipboard.writeText(traceId);
  };
  return (
    <span className="trace-chip">
      trace: <code>{shortTraceId(traceId)}</code>
      <button onClick={copy} aria-label="copy trace id">📋</button>
      <a href={`${jaeger}/trace/${traceId}`} target="_blank" rel="noreferrer">Jaeger</a>
    </span>
  );
}
