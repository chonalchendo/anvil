import { Glyph } from "../ui/Glyph";

export interface ReadoutProps {
  label: string;
  value: number;
  status?: string;
  note?: string;
}

// Grafana's stat, scaled down: a fixed-size neutral number, the glyph for state.
export function Readout({ label, value, status, note }: ReadoutProps) {
  return (
    <div className="flex flex-col gap-1 border-l border-line pl-4 first:border-l-0 first:pl-0">
      <span className="text-xs text-dim">{label}</span>
      <span className="flex items-center gap-2 text-xl font-semibold text-fg num">
        {status && <Glyph status={status} size={12} />}
        {value}
      </span>
      {note && <span className="text-xs text-faint">{note}</span>}
    </div>
  );
}
