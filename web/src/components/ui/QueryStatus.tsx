import { NotFound } from "../../lib/api";

export interface QueryStatusProps {
  error: Error | null;
  what: string;
}

// Loading and error are first-class: a failed query names what failed and
// what to do, instead of an endless "Loading…".
export function QueryStatus({ error, what }: QueryStatusProps) {
  if (!error) return <div className="p-10 text-dim" role="status" aria-live="polite">Loading {what}…</div>;
  if (error instanceof NotFound) return <div className="p-10 text-dim" role="alert">No {what} here. Check the address, or press ⌘K to find it.</div>;
  return (
    <div className="p-10 text-dim" role="alert">
      Could not load {what}: {error.message}. Check that the anvil web server is running, then reload.
    </div>
  );
}
