"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Activity,
  ArrowUp,
  Bell,
  Bot,
  ChevronLeft,
  ChevronRight,
  ScrollText,
  Settings,
  ShieldCheck,
} from "lucide-react";
import { MessageSquare } from "lucide-react";
import {
  openChatSocket,
  type ActivityEvent,
  type ChatTurn,
  type ServerMessage,
  type SocketHandle,
  type UIBlock,
} from "@/lib/ws";
import { RenderBlock } from "@/components/registry";
import { AuditView, SettingsView } from "@/components/admin";

type View = "chat" | "approvals" | "activity" | "audit" | "settings";

const SUGGESTIONS = [
  "Show vendors",
  "Create RFQ from ABC for 10 of Chair",
  "Show overdue invoices",
  "Show my purchase orders",
];

let turnSeq = 1;

export default function WorkspacePage() {
  const [view, setView] = useState<View>("chat");
  const [panelCollapsed, setPanelCollapsed] = useState(false);
  const [turns, setTurns] = useState<ChatTurn[]>([]);
  const [feed, setFeed] = useState<ActivityEvent[]>([]);
  const [input, setInput] = useState("");
  const [live, setLive] = useState(false);
  const [status, setStatus] = useState("");
  const [pendingApprovals, setPendingApprovals] = useState(0);

  const handleRef = useRef<SocketHandle | null>(null);
  const convRef = useRef(
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `conv-${Date.now()}`
  );
  const bottomRef = useRef<HTMLDivElement>(null);

  const latestPendingTurn = useMemo(() => {
    for (let i = turns.length - 1; i >= 0; i--) {
      if (turns[i].blocks.some((b) => b.type === "dry_run")) return turns[i].id;
    }
    return null;
  }, [turns]);

  const appendBlocks = useCallback((blocks: UIBlock[]) => {
    setTurns((prev) => {
      if (prev.length === 0) return prev;
      const last = prev[prev.length - 1];
      if (last.role !== "assistant") return prev;
      const next = [...prev];
      next[next.length - 1] = { ...last, blocks: [...last.blocks, ...blocks], pending: false };
      return next;
    });
  }, []);

  useEffect(() => {
    const handle = openChatSocket({
      tenantId: "default_tenant",
      conversationId: convRef.current,
      onOpen: () => setLive(true),
      onClose: () => setLive(false),
      onMessage: (msg: ServerMessage) => {
        if (msg.channel === "tenant-activity" && msg.type === "activity" && msg.event) {
          const ev = msg.event;
          setFeed((f) => [ev, ...f].slice(0, 100));
          if (ev.kind === "approval_pending") setPendingApprovals((n) => n + 1);
          if (ev.kind.startsWith("executed_")) setPendingApprovals((n) => Math.max(0, n - 1));
          return;
        }
        if (msg.channel !== "chat") return;
        switch (msg.type) {
          case "status":
            setStatus(msg.text || "");
            break;
          case "text":
            setStatus("");
            setTurns((t) => [
              ...t,
              { id: turnSeq++, role: "assistant", text: msg.text || "", blocks: [], pending: false },
            ]);
            break;
          case "ui":
            setStatus("");
            if (msg.blocks) appendBlocks(msg.blocks);
            break;
          case "done":
            setStatus("");
            setTurns((t) =>
              t.map((turn) => ({ ...turn, pending: false }))
            );
            break;
          case "error":
            setStatus("");
            setTurns((t) => [
              ...t,
              { id: turnSeq++, role: "assistant", text: msg.error || "Something went wrong.", blocks: [], pending: false, error: msg.error },
            ]);
            break;
        }
      },
    });
    handleRef.current = handle;
    return () => handle.close();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const send = useCallback(
    (text: string) => {
      const trimmed = text.trim();
      if (!trimmed) return;
      setTurns((t) => [
        ...t,
        { id: turnSeq++, role: "user", text: trimmed, blocks: [], pending: true },
      ]);
      setInput("");
      setStatus("Sending…");
      handleRef.current?.send(convRef.current, trimmed);
    },
    []
  );

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [turns, status]);

  const approvals = useMemo(
    () =>
      turns
        .filter((t) => t.blocks.some((b) => b.type === "dry_run"))
        .reverse(),
    [turns]
  );

  const nav: { id: View; label: string; icon: React.ReactNode; badge?: number }[] = [
    { id: "chat", label: "Chat", icon: <MessageSquare size={18} /> },
    { id: "approvals", label: "Approvals", icon: <ShieldCheck size={18} />, badge: pendingApprovals },
    { id: "activity", label: "Activity", icon: <Activity size={18} /> },
    { id: "audit", label: "Audit Trail", icon: <ScrollText size={18} /> },
    { id: "settings", label: "Settings", icon: <Settings size={18} /> },
  ];

  return (
    <div className="flex h-screen w-full bg-zinc-50 text-zinc-900 font-sans tracking-tight overflow-hidden">
      {/* Column 1: icon rail (64px) */}
      <nav className="flex w-16 shrink-0 flex-col items-center gap-1 bg-zinc-950 py-4 shadow-xl z-20">
        <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-zinc-800 to-zinc-900 border border-zinc-700 text-white shadow-sm">
          <Bot size={20} />
        </div>
        <div className="flex w-full flex-col gap-2 px-2">
          {nav.map((n) => (
            <button
              key={n.id}
              title={n.label}
              onClick={() => setView(n.id)}
              className={`relative flex h-11 w-full items-center justify-center rounded-xl transition-all duration-200 ${
                view === n.id ? "bg-zinc-800 text-white shadow-inner" : "text-zinc-500 hover:bg-zinc-800/50 hover:text-zinc-300"
              }`}
            >
              {view === n.id && (
                <span className="absolute left-0 top-1/2 -translate-y-1/2 h-6 w-1 rounded-r-full bg-emerald-500" />
              )}
              {n.icon}
              {!!n.badge && (
                <span className="absolute right-1 top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-amber-500 px-1 text-[9px] font-bold text-white shadow-sm">
                  {n.badge}
                </span>
              )}
            </button>
          ))}
        </div>
        {/* Bottom utility dock */}
        <div className="mt-auto flex flex-col items-center gap-3 w-full px-2">
          <button
            title="Notifications"
            onClick={() => setView("approvals")}
            className="relative flex h-11 w-full items-center justify-center rounded-xl text-zinc-500 transition-colors hover:bg-zinc-800/50 hover:text-zinc-300"
          >
            <Bell size={18} />
            {pendingApprovals > 0 && (
              <span className="absolute right-3 top-2 flex h-2.5 min-w-[2.5px] items-center justify-center rounded-full bg-rose-500 px-1 text-[9px] font-bold text-white shadow-sm">
                {pendingApprovals}
              </span>
            )}
          </button>
          <button
            title="Settings"
            onClick={() => setView("settings")}
            className={`flex h-11 w-full items-center justify-center rounded-xl transition-colors ${
              view === "settings" ? "bg-zinc-800 text-white shadow-inner" : "text-zinc-500 hover:bg-zinc-800/50 hover:text-zinc-300"
            }`}
          >
            <Settings size={18} />
          </button>
          <div title="Workspace (PRO)" className="relative mt-2 flex h-10 w-10 overflow-hidden items-center justify-center rounded-full bg-zinc-800 border border-zinc-700 text-white hover:ring-2 ring-zinc-700 transition-all cursor-pointer">
            <span className="text-[10px] font-bold tracking-wider">K</span>
            <span className="absolute -bottom-0.5 w-full text-center rounded-full bg-emerald-500 px-1 text-[7px] font-bold leading-tight text-white border border-zinc-950">
              PRO
            </span>
          </div>
        </div>
      </nav>

      {/* Column 2: sub-panel / stream (280px, collapsible) */}
      {!panelCollapsed && (
      <aside className="hidden w-[280px] shrink-0 flex-col border-r border-zinc-200 bg-zinc-50 lg:flex shadow-sm z-10">
        <div className="flex h-14 items-center justify-between border-b border-zinc-200 bg-white px-5">
          <span className="text-xs font-semibold tracking-tight text-zinc-900 flex items-center gap-2">
            <Activity size={15} className="text-zinc-500" />
            Live Activity
          </span>
          <div className="flex items-center gap-2">
            <span className={`h-1.5 w-1.5 rounded-full ${live ? "bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.5)]" : "bg-zinc-300"}`} />
            <button
              title="Collapse panel"
              onClick={() => setPanelCollapsed(true)}
              className="flex h-6 w-6 items-center justify-center rounded-md text-zinc-400 hover:bg-zinc-200 hover:text-zinc-700 transition-colors"
            >
              <ChevronLeft size={14} />
            </button>
          </div>
        </div>
        <div className="flex-1 space-y-3 overflow-y-auto bg-zinc-50 p-4">
          {feed.length === 0 && (
            <div className="flex flex-col items-center justify-center text-center p-4 pt-8">
               <Activity size={32} className="text-zinc-200 mb-3" />
               <p className="text-[12px] text-zinc-400 font-medium">
                 Stream connected.<br/>Awaiting system events...
               </p>
            </div>
          )}
          {feed.map((ev, i) => (
            <div key={`${ev.timestamp}-${i}`} className="rounded-xl border border-zinc-200 bg-white p-3.5 shadow-sm hover:shadow-md transition-shadow cursor-default">
              <div className="flex items-center gap-2 mb-1.5">
                <div className={`w-1.5 h-1.5 rounded-full ${
                  ev.kind === 'error' ? 'bg-rose-500' : 'bg-emerald-500'
                }`}></div>
                <h4 className="text-[11px] font-bold text-zinc-800 tracking-tight uppercase truncate">{ev.title}</h4>
              </div>
              {ev.detail && <p className="text-[12px] font-medium leading-relaxed text-zinc-500 pl-3.5">{ev.detail}</p>}
              <div className="mt-2.5 flex items-center justify-between pl-3.5">
                 <span className="font-mono text-[9px] uppercase font-bold text-zinc-400 bg-zinc-100 px-1.5 py-0.5 rounded">
                   {ev.kind.slice(0, 15)}
                 </span>
                 <span className="font-mono text-[9px] text-zinc-400">
                   {new Date(ev.timestamp * 1000).toLocaleTimeString()}
                 </span>
              </div>
            </div>
          ))}
        </div>
      </aside>
      )}
      {panelCollapsed && (
        <div className="hidden w-10 shrink-0 flex-col items-center border-r border-slate-200 bg-zinc-50 py-3 lg:flex">
          <button
            title="Expand panel"
            onClick={() => setPanelCollapsed(false)}
            className="flex h-6 w-6 items-center justify-center rounded-md text-slate-400 hover:bg-slate-200 hover:text-slate-700"
          >
            <ChevronRight size={14} />
          </button>
        </div>
      )}

      {/* Main */}
      <main className="flex min-w-0 flex-1 flex-col bg-white">
        {view === "chat" && (
          <>
            <div className="flex h-12 shrink-0 items-center justify-between border-b border-slate-100 px-5">
              <span className="flex items-center text-sm font-medium tracking-tight text-slate-800">
                <Bot size={16} className="mr-2 text-slate-400" /> Workspace Chat
              </span>
              <span className="flex items-center gap-2">
                <span className="rounded-full bg-slate-100 px-2 py-0.5 font-mono text-[10px] text-slate-500">
                  default_tenant
                </span>
                {live ? (
                  <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-600">
                    Connected
                  </span>
                ) : (
                  <span className="rounded-full bg-rose-50 px-2 py-0.5 text-[11px] font-medium text-rose-600">
                    Failed to fetch
                  </span>
                )}
              </span>
            </div>
            <div className="flex-1 space-y-4 overflow-y-auto p-5">
              {turns.length === 0 && (
                <div className="mx-auto max-w-lg pt-10 text-center">
                  <p className="mb-1 text-lg font-semibold tracking-tight">Talk to your ERP</p>
                  <p className="mb-5 text-sm text-slate-500">
                    Ask about vendors, orders, invoices — approve dry-runs by just replying “confirmed”.
                  </p>
                  <div className="flex flex-wrap justify-center gap-2">
                    {SUGGESTIONS.map((s) => (
                      <button
                        key={s}
                        onClick={() => send(s)}
                        className="rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-medium text-slate-600 hover:bg-slate-100"
                      >
                        {s}
                      </button>
                    ))}
                  </div>
                </div>
              )}
              {turns.map((t) => (
                <div key={t.id} className={`flex ${t.role === "user" ? "justify-end" : "justify-start"}`}>
                  <div className={`max-w-[85%] space-y-2 ${t.role === "user" ? "" : "w-full"}`}>
                    {t.text && (
                      <div
                        className={`inline-block rounded-xl px-3.5 py-2 text-sm leading-relaxed ${
                          t.role === "user"
                            ? "bg-slate-900 text-white"
                            : t.error
                              ? "border border-rose-200 bg-rose-50 text-rose-700"
                              : "border border-slate-200 bg-slate-50 text-slate-800"
                        }`}
                      >
                        {t.text}
                      </div>
                    )}
                    {t.blocks.map((b, i) => (
                      <RenderBlock
                        key={i}
                        block={b}
                        onAction={(text) => {
                          // Older dry-run cards are superseded by newer ones (single
                          // pending slot server-side) — route anyway; server holds latest.
                          send(text);
                        }}
                      />
                    ))}
                    {t.role === "assistant" && t.blocks.some((b) => b.type === "dry_run") && t.id !== latestPendingTurn && (
                      <p className="text-[11px] text-slate-400">Superseded by a newer request — reply in chat to continue.</p>
                    )}
                  </div>
                </div>
              ))}
              {status && <p className="text-xs text-slate-400">{status}…</p>}
              <div ref={bottomRef} />
            </div>
            <div className="shrink-0 border-t border-slate-100 p-4">
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  send(input);
                }}
                className="flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2 shadow-sm focus-within:ring-2 focus-within:ring-slate-200"
              >
                <input
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  placeholder="Ask anything about your business…"
                  className="flex-1 bg-transparent text-sm focus:outline-none placeholder:text-slate-400"
                />
                <button
                  type="submit"
                  className="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-900 text-white hover:bg-slate-700"
                >
                  <ArrowUp size={14} />
                </button>
              </form>
            </div>
          </>
        )}

        {view === "approvals" && (
          <div className="flex-1 overflow-y-auto p-8">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold tracking-tight text-slate-900">Approval Queue</h2>
              {live ? (
                <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-600">
                  Connected
                </span>
              ) : (
                <span className="rounded-full bg-rose-50 px-2 py-0.5 text-[11px] font-medium text-rose-600">
                  Failed to fetch
                </span>
              )}
            </div>
            {approvals.length === 0 && (
              <p className="text-sm text-slate-400">No dry-runs yet — they appear here as the AI proposes actions.</p>
            )}
            <div className="max-w-2xl space-y-3">
              {approvals.map((t) => (
                <div key={t.id} className={t.id === latestPendingTurn ? "" : "opacity-60"}>
                  {t.text && <p className="mb-1 text-sm text-slate-700">{t.text}</p>}
                  {t.blocks
                    .filter((b) => b.type === "dry_run")
                    .map((b, i) => (
                      <RenderBlock key={i} block={b} onAction={(text) => send(text)} />
                    ))}
                  {t.id !== latestPendingTurn && (
                    <p className="mt-1 text-[11px] text-slate-400">Superseded — only the latest dry-run is executable.</p>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {view === "activity" && (
          <div className="flex-1 overflow-y-auto p-8">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-lg font-semibold tracking-tight text-slate-900">Activity</h2>
              {live ? (
                <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-600">
                  Connected
                </span>
              ) : (
                <span className="rounded-full bg-rose-50 px-2 py-0.5 text-[11px] font-medium text-rose-600">
                  Failed to fetch
                </span>
              )}
            </div>
            {feed.length === 0 && <p className="text-sm text-slate-400">No events yet.</p>}
            <div className="max-w-2xl space-y-2">
              {feed.map((ev, i) => (
                <div key={`${ev.timestamp}-${i}`} className="rounded-lg border border-slate-200 p-3 text-sm">
                  <p className="font-medium text-slate-800">{ev.title}</p>
                  {ev.detail && <p className="text-xs text-slate-500">{ev.detail}</p>}
                  <p className="mt-1 font-mono text-[10px] text-slate-400">{ev.kind}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {view === "audit" && <AuditView />}

        {view === "settings" && <SettingsView />}
      </main>
    </div>
  );
}
