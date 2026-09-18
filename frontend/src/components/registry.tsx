"use client";

import React, { useMemo, useState } from "react";
import {
  AlertTriangle,
  BarChart3,
  CheckCircle2,
  ExternalLink,
  ShieldCheck,
  Table as TableIcon,
  X,
} from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { UIBlock } from "@/lib/ws";

// ---------------------------------------------------------------------------
// Citation popover: [1] badge -> verified ERP source card (ui.md blueprint)
// ---------------------------------------------------------------------------
export function CitationPopover({ id, record, label, url, irn }: {
  id?: string;
  record?: string;
  label?: string;
  url?: string;
  irn?: string;
}) {
  const ref = record || label || id || "record";
  return (
    <span className="group relative inline-block">
      <sup className="ml-0.5 inline-flex h-4 w-4 cursor-pointer items-center justify-center rounded-full bg-emerald-500/10 font-mono text-[10px] font-semibold text-emerald-600 transition-all hover:bg-emerald-500 hover:text-white">
        1
      </sup>
      <span className="absolute bottom-full left-1/2 z-50 mb-2 hidden w-72 -translate-x-1/2 rounded-xl border border-slate-200 bg-white p-3 text-left shadow-xl group-hover:block">
        <span className="flex items-center justify-between border-b border-slate-100 pb-2">
          <span className="flex items-center gap-1.5 text-xs font-medium text-emerald-600">
            <CheckCircle2 className="h-3.5 w-3.5" />
            <span>Verified Source</span>
          </span>
          <span className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] text-slate-600">
            Odoo ERP
          </span>
        </span>
        <span className="mt-2 block space-y-1 text-xs">
          <span className="block text-slate-500">
            Record: <span className="font-mono font-semibold text-slate-900">{ref}</span>
          </span>
          {irn && (
            <span className="block font-mono text-[11px] text-slate-500">IRN: {irn}</span>
          )}
        </span>
        {url && (
          <a
            href={url}
            target="_blank"
            rel="noreferrer"
            className="mt-2.5 flex items-center justify-between border-t border-slate-100 pt-2 text-[11px] text-slate-400 hover:text-slate-700"
          >
            <span>Open record in ERP</span>
            <ExternalLink className="h-3 w-3" />
          </a>
        )}
      </span>
    </span>
  );
}

// ---------------------------------------------------------------------------
// ResultTable: rows + columns derived from row keys, with a chart toggle that
// renders the first text column vs numeric columns (client-side GenUI).
// ---------------------------------------------------------------------------
function isNumeric(v: unknown): v is number {
  return typeof v === "number" && Number.isFinite(v);
}

export function ResultTable({ title, rows }: { title?: string; rows: Record<string, any>[] }) {
  const [view, setView] = useState<"table" | "chart">("table");
  const columns = useMemo(() => {
    const keys = new Set<string>();
    rows.forEach((r) => Object.keys(r).forEach((k) => {
      if (!["lines"].includes(k)) keys.add(k);
    }));
    return Array.from(keys).slice(0, 8);
  }, [rows]);

  const chart = useMemo(() => {
    if (rows.length === 0) return null;
    const labelKey = columns.find((c) => !isNumeric(rows[0][c])) ?? columns[0];
    const series = columns.filter((c) => rows.some((r) => isNumeric(r[c]))).slice(0, 3);
    if (!labelKey || series.length === 0) return null;
    return { labelKey, series };
  }, [rows, columns]);

  const palette = ["#0f172a", "#10b981", "#f59e0b"];

  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
      <div className="flex items-center justify-between border-b border-slate-100 px-4 py-2.5">
        <p className="text-xs font-semibold text-slate-800">{title || "Results"}</p>
        <div className="flex items-center gap-1 rounded-lg bg-slate-100 p-0.5">
          <button
            onClick={() => setView("table")}
            className={`flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium ${view === "table" ? "bg-white text-slate-900 shadow-sm" : "text-slate-500"}`}
          >
            <TableIcon size={12} /> Table
          </button>
          <button
            onClick={() => setView("chart")}
            disabled={!chart}
            className={`flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium ${view === "chart" ? "bg-white text-slate-900 shadow-sm" : "text-slate-500"} disabled:opacity-40`}
          >
            <BarChart3 size={12} /> Chart
          </button>
        </div>
      </div>
      {view === "table" || !chart ? (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50 text-left text-slate-500">
                {columns.map((c) => (
                  <th key={c} className="whitespace-nowrap px-3 py-2 font-medium">
                    {c.replace(/_/g, " ")}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {rows.map((row, i) => (
                <tr key={i} className="hover:bg-slate-50/60">
                  {columns.map((c) => (
                    <td key={c} className="max-w-[220px] truncate px-3 py-2 text-slate-700">
                      {typeof row[c] === "object" ? JSON.stringify(row[c]) : String(row[c] ?? "—")}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="h-64 p-3">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={rows}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
              <XAxis dataKey={chart.labelKey} tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              {chart.series.map((s, i) => (
                <Bar key={s} dataKey={s} fill={palette[i % palette.length]} radius={[4, 4, 0, 0]} />
              ))}
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Standalone chart block (for future backend-issued chart specs).
// Props: {kind: "bar"|"pie", data: [...], labelKey, series: [...]}
// ---------------------------------------------------------------------------
export function ChartBlock({ kind, data, labelKey, series }: {
  kind?: string;
  data: Record<string, any>[];
  labelKey: string;
  series: string[];
}) {
  const palette = ["#0f172a", "#10b981", "#f59e0b", "#3b82f6", "#8b5cf6"];
  return (
    <div className="rounded-xl border border-slate-200 bg-white p-3 shadow-sm">
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          {kind === "pie" ? (
            <PieChart>
              <Tooltip />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              <Pie data={data} dataKey={series[0]} nameKey={labelKey} outerRadius={90}>
                {data.map((_, i) => (
                  <Cell key={i} fill={palette[i % palette.length]} />
                ))}
              </Pie>
            </PieChart>
          ) : (
            <BarChart data={data}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
              <XAxis dataKey={labelKey} tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              {series.map((s, i) => (
                <Bar key={s} dataKey={s} fill={palette[i % palette.length]} radius={[4, 4, 0, 0]} />
              ))}
            </BarChart>
          )}
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// DryRunCard: impact preview + conversational approval (ui.md blueprint,
// light Quartr theme). Buttons send conversational replies ("confirmed" /
// "no, discard") so approval flows through the same pending mechanism.
// ---------------------------------------------------------------------------
export function DryRunCard({ props, onAction }: {
  props: {
    operation?: string;
    impact_summary?: string;
    risk_level?: string;
    confirmation_prompt?: string;
    confirmation_token?: string;
  };
  onAction: (text: string) => void;
}) {
  const high = (props.risk_level || "").toUpperCase() === "HIGH";
  return (
    <div className={`w-full rounded-xl border bg-white p-4 shadow-sm ${high ? "border-rose-200" : "border-amber-200"}`}>
      <div className="flex items-center justify-between border-b border-slate-100 pb-3">
        <div className="flex items-center gap-2">
          <span className={`flex h-7 w-7 items-center justify-center rounded-lg border ${high ? "border-rose-200 bg-rose-50 text-rose-500" : "border-amber-200 bg-amber-50 text-amber-500"}`}>
            <AlertTriangle className="h-4 w-4" />
          </span>
          <div>
            <h4 className="text-sm font-semibold tracking-tight text-slate-900">{props.operation || "Action"}</h4>
            <p className="text-xs text-slate-500">Dry-run preview — nothing executed yet</p>
          </div>
        </div>
        <span className={`rounded-full px-2.5 py-1 font-mono text-[11px] font-medium ${high ? "bg-rose-50 text-rose-600" : "bg-amber-50 text-amber-600"}`}>
          {props.risk_level || "MEDIUM"}
        </span>
      </div>
      <p className="my-3 text-xs leading-relaxed text-slate-600">{props.impact_summary}</p>
      {props.confirmation_prompt && (
        <p className="mb-3 rounded-lg bg-slate-50 px-3 py-2 text-xs font-medium text-slate-700">
          {props.confirmation_prompt}
        </p>
      )}
      <div className="flex items-center gap-2">
        <button
          onClick={() => onAction("no, discard it")}
          className="flex flex-1 items-center justify-center gap-1.5 rounded-lg border border-slate-200 bg-white py-2 text-xs font-medium text-slate-600 transition-all hover:bg-slate-50"
        >
          <X className="h-3.5 w-3.5 text-rose-500" /> Reject
        </button>
        <button
          onClick={() => onAction("confirmed, go on")}
          className="flex flex-1 items-center justify-center gap-1.5 rounded-lg bg-emerald-500 py-2 text-xs font-semibold text-white transition-all hover:bg-emerald-400"
        >
          <ShieldCheck className="h-3.5 w-3.5" /> Approve & Execute
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Registry: backend {type, props} -> live component.
// ---------------------------------------------------------------------------
export function RenderBlock({ block, onAction }: { block: UIBlock; onAction: (text: string) => void }) {
  switch (block.type) {
    case "table":
      return <ResultTable title={block.props.title} rows={block.props.rows || []} />;
    case "chart":
      return (
        <ChartBlock
          kind={block.props.kind}
          data={block.props.data || []}
          labelKey={block.props.labelKey}
          series={block.props.series || []}
        />
      );
    case "dry_run":
      return <DryRunCard props={block.props} onAction={onAction} />;
    case "citation":
      return (
        <div className="text-[11px] text-slate-400">
          Source <CitationPopover {...block.props} />
        </div>
      );
    case "approval":
      return (
        <div className="flex items-center gap-2">
          <button
            onClick={() => onAction("confirmed, go on")}
            className="rounded-lg bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white hover:bg-slate-700"
          >
            Confirm
          </button>
          <button
            onClick={() => onAction("no, discard it")}
            className="rounded-lg border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
          >
            Discard
          </button>
        </div>
      );
    default:
      return null;
  }
}
