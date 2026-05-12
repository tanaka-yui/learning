import { ApiError } from '../api/http';
import { TraceIdChip } from './TraceIdChip';

export function ErrorBanner({ error }: { error: unknown }) {
  if (!error) return null;
  if (error instanceof ApiError) {
    return (
      <div className="error-banner">
        <span>{error.message} <span className="muted">({error.code})</span></span>
        <span style={{ flex: 1 }} />
        <TraceIdChip traceId={error.traceId} />
      </div>
    );
  }
  const message = error instanceof Error ? error.message : String(error);
  return <div className="error-banner">{message}</div>;
}
