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
// SettingsView: runtime AI model + tenant ERP connectors.
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

type LLMSettings = {
  provider: string;
  base_url: string;
  model: string;
  max_tokens: number;
  has_api_key: boolean;
  updated_at?: string;
};

const LLM_PROVIDERS = [
  { value: "groq", label: "Groq" },
  { value: "openai", label: "OpenAI" },
  { value: "openrouter", label: "OpenRouter" },
  { value: "custom", label: "Custom OpenAI-compatible" },
];

const LLM_BASE_URLS: Record<string, string> = {
  groq: "https://api.groq.com/openai/v1",
  openai: "https://api.openai.com/v1",
  openrouter: "https://openrouter.ai/api/v1",
  custom: "",
};

const DEFAULT_LLM_FORM = {
  provider: "groq",
  base_url: LLM_BASE_URLS.groq,
  model: "llama3-70b-8192",
  max_tokens: 400,
  api_key: "",
};

export function SettingsView() {
  const [items, setItems] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ name: "default", erp_type: "MOCK", base_url: "", db_name: "", username: "", password: "" });
  const [testMsg, setTestMsg] = useState("");
  const [testing, setTesting] = useState(false);

  const [llm, setLlm] = useState<LLMSettings | null>(null);
  const [llmForm, setLlmForm] = useState(DEFAULT_LLM_FORM);
  const [llmLoading, setLlmLoading] = useState(true);
  const [llmSaving, setLlmSaving] = useState(false);
  const [llmTesting, setLlmTesting] = useState(false);
  const [llmMsg, setLlmMsg] = useState<{ tone: "ok" | "error" | "warn"; text: string } | null>(null);

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

  const loadLLM = useCallback(async () => {
    setLlmLoading(true);
    try {
      const res = await apiGet<{ data: LLMSettings }>("/settings/llm");
      const data = res.data;
      setLlm(data);
      setLlmForm({
        provider: data.provider || "groq",
        base_url: data.base_url || LLM_BASE_URLS[data.provider] || "",
        model: data.model || "",
        max_tokens: data.max_tokens || 400,
        api_key: "",
      });
    } catch (e: any) {
      setLlmMsg({ tone: "error", text: e.message || "Failed to load AI model settings" });
    } finally {
      setLlmLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    loadLLM();
  }, [load, loadLLM]);

  const payload = () => ({
    name: form.name || "default",
    erp_type: form.erp_type,
    base_url: form.base_url || undefined,
    db_name: form.db_name || undefined,
    username: form.username || undefined,
    password: form.password || undefined,
  });

  const llmPayload = () => ({
    provider: llmForm.provider,
    base_url: llmForm.base_url,
    model: llmForm.model,
    max_tokens: Number(llmForm.max_tokens) || 400,
    api_key: llmForm.api_key,
  });

  const onProviderChange = (provider: string) => {
    setLlmForm((prev) => {
      const knownDefaults = Object.values(LLM_BASE_URLS).filter(Boolean);
      const shouldReplace = !prev.base_url.trim() || knownDefaults.includes(prev.base_url.trim());
      return {
        ...prev,
        provider,
        base_url: shouldReplace ? LLM_BASE_URLS[provider] || "" : prev.base_url,
      };
    });
  };

  const onSaveLLM = async () => {
    setLlmSaving(true);
    setLlmMsg(null);
    try {
      const res = await apiPost<{ data: LLMSettings; applied: boolean; warning?: string }>("/settings/llm", llmPayload());
      setLlm(res.data);
      setLlmForm((prev) => ({
        ...prev,
        provider: res.data.provider,
        base_url: res.data.base_url,
        model: res.data.model,
        max_tokens: res.data.max_tokens,
        api_key: "",
      }));
      if (res.applied === false) {
        setLlmMsg({ tone: "warn", text: res.warning || "Saved, but the AI engine has not applied it yet." });
      } else {
        setLlmMsg({ tone: "ok", text: "AI model settings saved and applied." });
      }
    } catch (e: any) {
      setLlmMsg({ tone: "error", text: e.message || "Save failed" });
    } finally {
      setLlmSaving(false);
    }
  };

  const onTestLLM = async () => {
    setLlmTesting(true);
    setLlmMsg(null);
    try {
      const res = await apiPost<{ ok: boolean; message: string }>("/settings/llm/test", llmPayload());
      setLlmMsg({ tone: res.ok ? "ok" : "error", text: res.message });
    } catch (e: any) {
      setLlmMsg({ tone: "error", text: e.message || "Test failed" });
    } finally {
      setLlmTesting(false);
    }
  };

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
  const messageTone = (tone: "ok" | "error" | "warn") =>
    tone === "ok" ? "text-emerald-700" : tone === "warn" ? "text-amber-700" : "text-rose-600";

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mb-6 max-w-2xl rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-1 flex items-center justify-between">
          <h2 className="text-lg font-semibold tracking-tight">AI model</h2>
          <span
            className={`rounded-full px-2.5 py-1 text-[11px] font-medium ${
              llm?.has_api_key ? "bg-emerald-50 text-emerald-600" : "bg-amber-50 text-amber-700"
            }`}
          >
            {llm?.has_api_key ? "API key configured" : "No API key"}
          </span>
        </div>
        <p className="mb-4 text-xs text-slate-500">
          The agent uses this provider for every conversation. The key is encrypted at rest and applied without restarting the AI engine.
        </p>

        {llmLoading ? (
          <p className="text-sm text-slate-400">Loading…</p>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-3">
              <select className={input} value={llmForm.provider} onChange={(e) => onProviderChange(e.target.value)}>
                {LLM_PROVIDERS.map((p) => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
              <input
                className={input}
                placeholder="Model, e.g. llama3-70b-8192"
                value={llmForm.model}
                onChange={(e) => setLlmForm({ ...llmForm, model: e.target.value })}
              />
              <input
                className={`${input} col-span-2`}
                placeholder="Base URL, e.g. https://api.groq.com/openai/v1"
                value={llmForm.base_url}
                onChange={(e) => setLlmForm({ ...llmForm, base_url: e.target.value })}
              />
              <input
                className={input}
                type="number"
                min={1}
                max={8192}
                placeholder="Max output tokens"
                value={llmForm.max_tokens}
                onChange={(e) => setLlmForm({ ...llmForm, max_tokens: Number(e.target.value) })}
              />
              <input
                className={input}
                type="password"
                placeholder={llm?.has_api_key ? "Leave blank to keep the current key" : "API key"}
                value={llmForm.api_key}
                onChange={(e) => setLlmForm({ ...llmForm, api_key: e.target.value })}
              />
            </div>
            <p className="mt-2 text-[11px] text-slate-400">
              {llm?.has_api_key
                ? `Current model: ${llm.model} · Key stored encrypted`
                : "Paste the provider API key once; it is stored encrypted in Postgres."}
            </p>
            {llmMsg && <p className={`mt-3 text-xs ${messageTone(llmMsg.tone)}`}>{llmMsg.text}</p>}
            <div className="mt-3 flex gap-2">
              <button
                onClick={onTestLLM}
                disabled={llmTesting}
                className="rounded-lg border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              >
                {llmTesting ? "Testing…" : "Test provider"}
              </button>
              <button
                onClick={onSaveLLM}
                disabled={llmSaving}
                className="rounded-lg bg-slate-900 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
              >
                {llmSaving ? "Saving…" : "Save AI model"}
              </button>
            </div>
          </>
        )}
      </div>

      <h2 className="mb-4 text-lg font-semibold tracking-tight">ERP connector</h2>
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
              <input className={`${input} col-span-2`} placeholder="Database URL (e.g. postgres://user:pass@10.0.0.25:5432/db)" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
              <input className={`${input} col-span-2`} type="password" placeholder="Additional credentials (if not in URL)" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </>
          ) : (
            <>
              <input className={input} placeholder="http://192.168.1.50:8069" value={form.base_url} onChange={(e) => setForm({ ...form, base_url: e.target.value })} />
              <input className={input} placeholder="Database / tenant ID" value={form.db_name} onChange={(e) => setForm({ ...form, db_name: e.target.value })} />
              <input className={input} placeholder="Username / client ID" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              <input className={input} type="password" placeholder="Password / API key" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </>
          )}
        </div>
        <p className="mt-2 text-[11px] text-slate-400">
          Use the Odoo server&apos;s IP or hostname reachable from this deployment. Do not use host.docker.internal.
        </p>
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
        <p className="mt-3 text-[11px] text-slate-400">Connector secrets persist in Postgres in v1 — move to Vault before multi-customer production.</p>
      </div>
    </div>
  );
}
