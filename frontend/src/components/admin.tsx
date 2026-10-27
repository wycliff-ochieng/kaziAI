"use client";

import React, { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Download, RefreshCw, ShieldAlert, Trash2 } from "lucide-react";
import { apiDelete, apiGet, apiPost, exportUrl } from "@/lib/api";

// ---------------------------------------------------------------------------
// AuditView: hash-chained trail + verify + CSV export.
// ---------------------------------------------------------------------------
type AuditEvent = {
  id: number;
  timestamp: string;
  tenant_id: string;
  actor: string;
  channel: string;
  prompt: string;
  intent: string;
  target?: string;
  approval: string;
  hash: string;
};

export function AuditView() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [verify, setVerify] = useState<{ ok?: boolean; checked?: number }>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [list, v] = await Promise.all([
        apiGet<{ data: AuditEvent[] }>("/audit?limit=100"),
        apiGet<{ ok: boolean; checked: number }>("/audit/verify"),
      ]);
      setEvents(list.data || []);
      setVerify({ ok: v.ok, checked: v.checked });
    } catch (e: any) {
      setError(e.message || "Failed to load audit trail");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold tracking-tight">Audit Trail</h2>
        <div className="flex items-center gap-2">
          {verify.ok !== undefined && (
            <span
              className={`flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium ${
                verify.ok ? "bg-emerald-50 text-emerald-600" : "bg-rose-50 text-rose-600"
              }`}
            >
              {verify.ok ? <CheckCircle2 size={12} /> : <ShieldAlert size={12} />}
              {verify.ok ? `Chain verified (${verify.checked})` : "Chain TAMPERED"}
            </span>
          )}
          <button
            onClick={load}
            className="flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
          >
            <RefreshCw size={12} /> Refresh
          </button>
          <a
            href={exportUrl("/audit/export")}
            className="flex items-center gap-1 rounded-lg bg-slate-900 px-2.5 py-1.5 text-xs font-semibold text-white hover:bg-slate-700"
          >
            <Download size={12} /> CSV
          </a>
        </div>
      </div>
      {loading && <p className="text-sm text-slate-400">Loading…</p>}
      {error && <p className="text-sm text-rose-600">{error}</p>}
      {!loading && !error && events.length === 0 && (
        <p className="text-sm text-slate-400">No audit rows yet — chat with the workspace first.</p>
      )}
      {events.length > 0 && (
        <div className="overflow-x-auto rounded-xl border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-slate-100 bg-slate-50 text-left text-slate-500">
                <th className="px-3 py-2 font-medium">Time</th>
                <th className="px-3 py-2 font-medium">Prompt</th>
                <th className="px-3 py-2 font-medium">Intent</th>
                <th className="px-3 py-2 font-medium">Target</th>
                <th className="px-3 py-2 font-medium">Approval</th>
                <th className="px-3 py-2 font-medium">Hash</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {events.map((e) => (
                <tr key={e.id} className="hover:bg-slate-50/60">
                  <td className="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-500">
                    {new Date(e.timestamp).toLocaleString()}
                  </td>
                  <td className="max-w-[280px] truncate px-3 py-2 text-slate-700">{e.prompt}</td>
                  <td className="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-600">{e.intent}</td>
                  <td className="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-600">{e.target || "—"}</td>
                  <td className="whitespace-nowrap px-3 py-2">
                    <span
                      className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                        e.approval === "executed"
                          ? "bg-emerald-50 text-emerald-600"
                          : e.approval === "pending"
                            ? "bg-amber-50 text-amber-600"
                            : e.approval === "rejected"
                              ? "bg-rose-50 text-rose-600"
                              : "bg-slate-100 text-slate-500"
                      }`}
                    >
                      {e.approval}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-400">
                    {e.hash.slice(0, 10)}…
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// SettingsView: tenant ERP connectors (list / add / test / delete).
// ---------------------------------------------------------------------------
type Connector = {
  tenant_id: string;
  name: string;
  erp_type: string;
  base_url?: string;
  db_name?: string;
  username?: string;
  has_secret?: boolean;
};

export function SettingsView() {
  const [items, setItems] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ name: "default", erp_type: "MOCK", base_url: "", db_name: "", username: "", password: "" });
  const [testMsg, setTestMsg] = useState("");
  const [testing, setTesting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await apiGet<{ data: Connector[] }>("/connectors");
      setItems(res.data || []);
    } catch (e: any) {
      setError(e.message || "Failed to load connectors");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const payload = () => ({
    name: form.name || "default",
    erp_type: form.erp_type,
    base_url: form.base_url || undefined,
    db_name: form.db_name || undefined,
    username: form.username || undefined,
    password: form.password || undefined,
  });

  const onTest = async () => {
    setTesting(true);
    setTestMsg("");
    try {
      const res = await apiPost<{ ok: boolean; version?: string; message: string }>("/connectors/test", payload());
      setTestMsg(`${res.ok ? "✓" : "✗"} ${res.message}`);
    } catch (e: any) {
      setTestMsg(`✗ ${e.message}`);
    } finally {
      setTesting(false);
    }
  };

  const onSave = async () => {
    setError("");
    try {
      await apiPost("/connectors", payload());
      setForm({ name: "default", erp_type: "MOCK", base_url: "", db_name: "", username: "", password: "" });
      await load();
    } catch (e: any) {
      setError(e.message || "Save failed");
    }
  };

  const onDelete = async (name: string) => {
    try {
      await apiDelete(`/connectors/${encodeURIComponent(name)}`);
      await load();
    } catch (e: any) {
      setError(e.message || "Delete failed");
    }
  };

  const onTestStored = async (name: string) => {
    setTestMsg("");
    try {
      const res = await apiPost<{ ok: boolean; version?: string; message: string }>(
        `/connectors/${encodeURIComponent(name)}/test`
      );
      setTestMsg(`${name}: ${res.ok ? "✓" : "✗"} ${res.message}`);
    } catch (e: any) {
      setTestMsg(`${name}: ✗ ${e.message}`);
    }
  };

  const input = "w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-slate-200";

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <h2 className="mb-4 text-lg font-semibold tracking-tight">Connectors</h2>
      {error && <p className="mb-3 text-sm text-rose-600">{error}</p>}
      {loading ? (
        <p className="text-sm text-slate-400">Loading…</p>
      ) : (
        <div className="mb-6 max-w-2xl space-y-2">
          {items.length === 0 && <p className="text-sm text-slate-400">No connectors yet — chat uses environment defaults.</p>}
          {items.map((c) => (
            <div key={c.name} className="flex items-center justify-between rounded-xl border border-slate-200 p-3">
              <div>
                <p className="text-sm font-semibold text-slate-800">
                  {c.name} <span className="ml-1 rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[10px] text-slate-600">{c.erp_type}</span>
                </p>
                <p className="font-mono text-[11px] text-slate-500">
                  {[c.base_url, c.db_name, c.username].filter(Boolean).join(" · ") || "environment defaults"}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => onTestStored(c.name)}
                  className="rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-50"
                >
                  Test
                </button>
                <button
                  onClick={() => onDelete(c.name)}
                  className="flex items-center gap-1 rounded-lg border border-slate-200 px-2.5 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50"
                >
                  <Trash2 size={12} /> Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      <div className="max-w-2xl rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <h3 className="mb-3 text-sm font-semibold tracking-tight text-slate-800">Add / replace connector</h3>
        <div className="grid grid-cols-2 gap-3">
          <input className={input} placeholder="Name (default)" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          <select className={input} value={form.erp_type} onChange={(e) => setForm({ ...form, erp_type: e.target.value })}>
            <option value="MOCK">MOCK</option>
            <option value="ODOO">ODOO</option>
            <option value="POSTGRES">POSTGRES DB</option>
            <option value="MYSQL">MYSQL DB</option>
            <option value="SAP">SAP</option>
            <option value="DYNAMICS">DYNAMICS 365</option>
          </select>
          {["POSTGRES", "MYSQL"].includes(form.erp_type) ? (
            <>
              <input className={`${input} col-span-2`} placeholder="Database URL (e.g. postgres://user:pass@host/db)" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
              <input className={`${input} col-span-2`} type="password" placeholder="Additional Credentials (if not in URL)" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </>
          ) : (
            <>
              <input className={input} placeholder="Base URL / API endpoint" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
              <input className={input} placeholder="Database / Tenant ID" value={form.db_name} onChange={(e) => setForm({ ...form, db_name: e.target.value })} />
              <input className={input} placeholder="Username / Client ID" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              <input className={input} type="password" placeholder="Password / API key" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </>
          )}
        </div>
        {testMsg && <p className="mt-3 text-xs text-slate-600">{testMsg}</p>}
        <div className="mt-3 flex gap-2">
          <button
            onClick={onTest}
            disabled={testing}
            className="rounded-lg border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
          >
            {testing ? "Testing…" : "Test connection"}
          </button>
          <button onClick={onSave} className="rounded-lg bg-slate-900 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700">
            Save connector
          </button>
        </div>
        <p className="mt-3 text-[11px] text-slate-400">Secrets persist in Postgres in v1 — move to Vault before multi-customer production.</p>
      </div>
    </div>
  );
}
