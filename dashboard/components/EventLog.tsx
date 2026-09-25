"use client";

import type { LogEntry } from "@/lib/useEventLog";

interface EventLogProps {
  entries: LogEntry[];
}

export default function EventLog({ entries }: EventLogProps) {
  return (
    <div className="flex min-h-0 flex-1 flex-col rounded-lg border border-neutral-800 bg-[#171310] p-4">
      <h2 className="mb-2 shrink-0 text-xs font-semibold uppercase tracking-wider text-amber-100/50">
        Event Log
      </h2>
      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        {entries.length === 0 ? (
          <p className="text-xs text-amber-100/30">Waiting for events…</p>
        ) : (
          <ul className="flex flex-col gap-1.5 text-xs">
            {entries.map((e) => (
              <li key={e.id} className="flex gap-2 border-b border-neutral-800/60 pb-1.5">
                <span className="shrink-0 font-mono text-amber-100/40">
                  {e.ts}
                </span>
                <span className="text-amber-50/85">{e.text}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
