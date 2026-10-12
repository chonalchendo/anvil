import { Fragment } from "react";

// The server marks hits with \u0001…\u0002 so the snippet stays plain text
// until here; nothing from a body is ever parsed as HTML.
export function Snippet({ text }: { text: string }) {
  return text.split(/(\u0001[^\u0002]*\u0002)/).map((p, i) =>
    p.startsWith("\u0001") ? <mark key={i}>{p.slice(1, -1)}</mark> : <Fragment key={i}>{p}</Fragment>,
  );
}
