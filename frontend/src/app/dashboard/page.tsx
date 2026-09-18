"use client";

import React, { useState, useEffect, useRef } from "react";
import { 
  Home, 
  MessageSquare, 
  CheckCircle, 
  Activity, 
  Blocks, 
  Settings, 
  Bell,
  User,
  Send,
  Bot
} from "lucide-react";
import { 
  openChatSocket, 
  SocketHandle, 
  ServerMessage, 
  ChatTurn, 
  ActivityEvent 
} from "@/lib/ws";

export default function DashboardAppShell() {
  const [activeTab, setActiveTab] = useState<"chat" | "approvals" | "integrations">("chat");
  const [socketStatus, setSocketStatus] = useState<"connecting" | "connected" | "failed">("connecting");
  const [inputText, setInputText] = useState("");
  const [chatHistory, setChatHistory] = useState<ChatTurn[]>([]);
  const [activityStream, setActivityStream] = useState<ActivityEvent[]>([
    { 
      kind: "system", 
      title: "Workspace Authorized", 
      detail: "Secure channel established to ERP Gateway.", 
      tenant_id: "default_tenant", 
      timestamp: Date.now() 
    }
  ]);

  const socketRef = useRef<SocketHandle | null>(null);
  const nextTurnId = useRef<number>(1);
  const chatScrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll chat to bottom
  useEffect(() => {
    if (chatScrollRef.current) {
      chatScrollRef.current.scrollTop = chatScrollRef.current.scrollHeight;
    }
  }, [chatHistory]);

  // Establish WebSocket connection
  useEffect(() => {
    const handle = openChatSocket({
      tenantId: "default_tenant",
      conversationId: "demo-web-1",
      onOpen: () => setSocketStatus("connected"),
      onClose: () => setSocketStatus("failed"),
      onMessage: (msg: ServerMessage) => {
        if (msg.channel === "chat") {
          setChatHistory(prev => {
            let newHist = [...prev];
            let last = newHist[newHist.length - 1];
            
            // If there's no running assistant turn, add one
            if (!last || last.role === "user" || last.status === "done") {
              last = { id: nextTurnId.current++, role: "assistant", text: "", blocks: [], pending: true };
              newHist.push(last);
            }
            
            if (msg.type === "text" && msg.text) {
              last.text += msg.text;
            }
            if (msg.type === "ui" && msg.blocks) {
              last.blocks = [...(last.blocks || []), ...msg.blocks];
            }
            if (msg.type === "done") {
              last.status = "done";
              last.pending = false;
            }
            if (msg.type === "error" && msg.error) {
              last.error = msg.error;
              last.pending = false;
            }
            
            return newHist;
          });
        } else if (msg.channel === "tenant-activity" && msg.event) {
          setActivityStream(prev => [msg.event!, ...prev]);
        }
      }
    });
    
    socketRef.current = handle;
    return () => {
      handle.close();
    };
  }, []);

  const handleSend = () => {
    if (!inputText.trim()) return;
    const textMsg = inputText.trim();
    
    // Add user msg optimistically
    setChatHistory(prev => [
      ...prev, 
      { id: nextTurnId.current++, role: "user", text: textMsg, blocks: [], pending: false, status: "done" }
    ]);
    
    socketRef.current?.send("demo-web-1", textMsg);
    setInputText("");
  };

  const NavItem = ({ icon: Icon, tabId, label }: { icon: any, tabId: any, label: string }) => {
    const isActive = activeTab === tabId;
    return (
      <button 
        onClick={() => setActiveTab(tabId)}
        title={label}
        className={`relative w-12 h-12 flex items-center justify-center rounded-xl mx-auto transition-all ${
          isActive ? "bg-zinc-800 text-white" : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900"
        }`}
      >
        {isActive && (
          <div className="absolute left-[-16px] top-1/2 -translate-y-1/2 w-1 h-6 bg-emerald-500 rounded-r-full" />
        )}
        <Icon size={20} strokeWidth={isActive ? 2.5 : 2} />
      </button>
    );
  };

  return (
    <div className="flex h-screen w-full bg-zinc-950 font-sans text-zinc-900 overflow-hidden">
      
      {/* COLUMN 1: Slim Icon Rail */}
      <nav className="w-16 shrink-0 h-full flex flex-col justify-between py-4 border-r border-zinc-900">
        <div className="space-y-4 flex flex-col items-center">
          {/* Brand Logo */}
          <div className="w-10 h-10 rounded-full bg-gradient-to-br from-zinc-800 to-zinc-900 border border-zinc-700 flex items-center justify-center mb-4">
            <Bot size={22} className="text-zinc-200" />
          </div>
          
          <NavItem icon={Home} label="Home" tabId="home" />
          <NavItem icon={MessageSquare} label="Workspace Chat" tabId="chat" />
          <NavItem icon={CheckCircle} label="Pending Approvals" tabId="approvals" />
          <NavItem icon={Blocks} label="Integrations" tabId="integrations" />
        </div>
        
        <div className="space-y-4 flex flex-col items-center">
          <button className="relative w-10 h-10 flex items-center justify-center rounded-full text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900 transition-colors">
            <Bell size={18} />
            <span className="absolute top-2 right-2.5 w-1.5 h-1.5 bg-emerald-500 rounded-full"></span>
          </button>
          <button className="w-10 h-10 flex items-center justify-center rounded-full text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900 transition-colors">
            <Settings size={18} />
          </button>
          <button className="w-10 h-10 rounded-full bg-zinc-800 border border-zinc-700 flex items-center justify-center overflow-hidden hover:ring-2 ring-zinc-700 transition-all">
            <span className="text-[9px] font-bold text-white tracking-wider">PRO</span>
          </button>
        </div>
      </nav>

      {/* COLUMN 2: Secondary Stream / Panel */}
      <aside className="w-[280px] shrink-0 bg-zinc-50 h-full border-r border-zinc-200 flex flex-col z-10 shadow-sm relative">
        <div className="h-14 border-b border-zinc-200 px-5 flex items-center justify-between bg-white shrink-0">
          <h2 className="font-semibold text-sm tracking-tight text-zinc-900 flex items-center gap-2">
            <Activity size={16} className="text-zinc-500" />
            Live Activity
          </h2>
          <span className="px-2 py-0.5 rounded-full bg-emerald-100/50 text-emerald-700 text-[10px] font-bold tracking-widest uppercase">Stream</span>
        </div>
        
        <div className="flex-1 overflow-y-auto p-4 space-y-3 bg-zinc-50/50">
          {activityStream.map((ev, i) => (
            <div key={i} className="p-3.5 bg-white border border-zinc-200 rounded-xl shadow-[0_1px_2px_rgba(0,0,0,0.02)] hover:shadow-sm transition-all text-left">
              <div className="flex items-center gap-2 mb-1">
                <div className={`w-2 h-2 rounded-full ${
                  ev.kind === 'error' ? 'bg-red-500' : 
                  ev.kind === 'system' ? 'bg-blue-500' : 
                  'bg-emerald-500'
                }`}></div>
                <h4 className="text-xs font-semibold text-zinc-800 tracking-tight">{ev.title}</h4>
              </div>
              <p className="text-[12px] leading-relaxed text-zinc-500 pl-4">{ev.detail}</p>
              {ev.record_id && (
                <div className="pl-4 mt-2">
                  <span className="text-[10px] font-mono text-zinc-500 bg-zinc-100 py-0.5 px-1.5 rounded">{ev.record_id}</span>
                </div>
              )}
              <span className="block pl-4 text-[10px] text-zinc-400 mt-2 font-medium">
                {new Date(ev.timestamp).toLocaleTimeString()}
              </span>
            </div>
          ))}
        </div>
      </aside>

      {/* COLUMN 3: Main Workspace Canvas */}
      <main className="flex-1 bg-white flex flex-col relative h-full"> 
        {/* Top Header */}
        <header className="h-14 px-6 border-b border-zinc-200 flex items-center justify-between shrink-0 bg-white">
          <h1 className="text-sm font-semibold tracking-tight">
            {activeTab === 'chat' && 'AI Workspace Chat'}
            {activeTab === 'approvals' && 'Pending Dry-Run Approvals'}
            {activeTab === 'integrations' && 'ERP Connectors & Apps'}
          </h1>
          <div className="flex items-center gap-3">
             <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-zinc-100">
                <div className={`w-1.5 h-1.5 rounded-full ${
                  socketStatus === 'connected' ? 'bg-emerald-500' : 
                  socketStatus === 'connecting' ? 'bg-amber-400 animate-pulse' : 'bg-red-500'
                }`}></div>
                <span className="text-[10px] font-semibold tracking-wider uppercase text-zinc-600">
                  {socketStatus}
                </span>
             </div>
          </div>
        </header>

        {/* Content Area Rendering Based on Active Tab */}
        <div className="flex-1 overflow-hidden relative">
          
          {/* TAB: Chat Workspace */}
          {activeTab === 'chat' && (
            <div className="flex flex-col h-full bg-[#fdfdfd]">
              {/* Background gradient hint */}
              <div className="absolute inset-0 bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [bg-size:16px_16px] opacity-[0.35] pointer-events-none"></div>

              {/* Chat View */}
              <div ref={chatScrollRef} className="flex-1 overflow-y-auto p-4 md:p-8 space-y-6 z-10 scroll-smooth">
                {chatHistory.length === 0 && (
                  <div className="h-full flex flex-col items-center justify-center text-center opacity-70">
                    <Bot size={48} className="text-zinc-300 mb-4" />
                    <h3 className="text-lg font-medium tracking-tight text-zinc-700">How can I assist your operations?</h3>
                    <p className="text-xs text-zinc-500 mt-2 max-w-sm">Use natural language to query inventory, approve POs, or reconcile M-Pesa receipts.</p>
                  </div>
                )}
                
                {chatHistory.map(turn => (
                  <div key={turn.id} className={`flex ${turn.role === 'user' ? 'justify-end' : 'justify-start'} w-full`}>
                    <div className={`
                      max-w-[85%] lg:max-w-[70%] text-sm rounded-xl px-4 py-3 shadow-sm
                      ${turn.role === 'user' 
                        ? 'bg-zinc-900 text-white rounded-tr-[4px]' 
                        : 'bg-white border border-zinc-200 text-zinc-800 rounded-tl-[4px]'
                      }
                    `}>
                      {turn.text && <p className="leading-relaxed whitespace-pre-wrap">{turn.text}</p>}
                      
                      {turn.role === 'assistant' && turn.pending && !turn.text && (
                        <div className="flex items-center gap-1.5 py-1 text-zinc-400 h-6">
                           <span className="w-1.5 h-1.5 rounded-full border border-zinc-400 animate-bounce"></span>
                           <span className="w-1.5 h-1.5 rounded-full bg-zinc-300 animate-bounce" style={{animationDelay: '150ms'}}></span>
                           <span className="w-1.5 h-1.5 rounded-full bg-zinc-300 animate-bounce" style={{animationDelay: '300ms'}}></span>
                        </div>
                      )}

                      {/* Render UI Blocks if any */}
                      { turn.blocks?.map((blk, idx) => (
                          <div key={idx} className="mt-3 bg-zinc-50 border border-zinc-100 rounded-lg p-3 text-xs">
                             <div className="text-[10px] uppercase font-bold tracking-widest text-zinc-400 mb-2 border-b border-zinc-200 pb-1">
                               Dynamic UI: {blk.type}
                             </div>
                             <pre className="font-mono text-[10px] overflow-auto text-zinc-600">
                               {JSON.stringify(blk.props, null, 2)}
                             </pre>
                          </div>
                      ))}

                      {turn.error && (
                         <div className="mt-2 text-xs text-red-500 font-medium bg-red-50 p-2 rounded border border-red-100">
                           Error: {turn.error}
                         </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>

              {/* Chat Input */}
              <div className="shrink-0 p-4 bg-white border-t border-zinc-200 z-10 flex flex-col items-center">
                <div className="w-full max-w-4xl relative">
                  <input
                    type="text"
                    value={inputText}
                    onChange={(e) => setInputText(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) { 
                        e.preventDefault(); 
                        handleSend(); 
                      }
                    }}
                    placeholder="Ask anything about your operations..."
                    className="w-full bg-white border border-zinc-300 rounded-xl pl-4 pr-12 py-3.5 text-sm focus:border-zinc-900 focus:ring-1 focus:ring-zinc-900 transition-all shadow-sm placeholder:text-zinc-400"
                  />
                  <button 
                    onClick={handleSend}
                    disabled={!inputText.trim()}
                    className="absolute right-2 top-1/2 -translate-y-1/2 w-9 h-9 flex items-center justify-center rounded-lg bg-zinc-900 hover:bg-zinc-800 disabled:bg-zinc-200 text-white transition-all shadow-sm"
                  >
                    <Send size={16} className={inputText.trim() ? "translate-x-[-1px] translate-y-[1px]" : ""} />
                  </button>
                </div>
                <div className="w-full max-w-4xl text-center mt-3">
                   <span className="text-[10px] font-medium text-zinc-400 tracking-wide">
                     AI Assistants can make mistakes. Verify critical actions before approval.
                   </span>
                </div>
              </div>
            </div>
          )}

          {/* TAB: Integrations Placeholder */}
          {activeTab === 'integrations' && (
            <div className="p-8 h-full bg-[#fdfdfd] z-10 relative overflow-y-auto">
               <h2 className="text-xl font-bold tracking-tight mb-6">ERP Connectors & Apps</h2>
               <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                 {[
                   { name: "Odoo v19", status: "Connected", code: "odoo_19", icon: "📦" },
                   { name: "SAP Business One", status: "Available", code: "sap_b1", icon: "🏢" },
                   { name: "M-Pesa Daraja", status: "Configured", code: "mpesa", icon: "💸" }
                 ].map(app => (
                   <div key={app.code} className="bg-white border border-zinc-200 rounded-xl p-5 flex flex-col shadow-sm">
                      <div className="flex justify-between items-start mb-4">
                        <div className="w-10 h-10 rounded-lg bg-zinc-100 flex items-center justify-center text-xl">
                          {app.icon}
                        </div>
                        <span className={`text-[10px] font-bold uppercase tracking-wider px-2 py-1 rounded-md ${
                          app.status === 'Connected' || app.status === 'Configured' ? 'bg-emerald-100 text-emerald-700' : 'bg-zinc-100 text-zinc-600'
                        }`}>
                          {app.status}
                        </span>
                      </div>
                      <h3 className="font-semibold text-sm mb-1">{app.name}</h3>
                      <p className="text-xs text-zinc-500 mb-4 flex-1">Configure your API credentials to connect this app to Kazi AI.</p>
                      <button className="w-full py-2 bg-zinc-900 hover:bg-zinc-800 text-white rounded-lg text-xs font-medium transition-colors">
                        Manage Connection
                      </button>
                   </div>
                 ))}
               </div>
            </div>
          )}
          
          {/* TAB: Approvals Placeholder */}
          {activeTab === 'approvals' && (
            <div className="p-8 h-full bg-[#fdfdfd] z-10 relative overflow-y-auto flex items-center justify-center text-center">
                <div>
                   <CheckCircle className="mx-auto text-zinc-300 w-16 h-16 mb-4" />
                   <h2 className="text-lg font-semibold tracking-tight text-zinc-800">No Pending Approvals</h2>
                   <p className="text-sm text-zinc-500 mt-2">All dry-run write operations have been processed.</p>
                </div>
            </div>
          )}

        </div>
      </main>
    </div>
  );
}
