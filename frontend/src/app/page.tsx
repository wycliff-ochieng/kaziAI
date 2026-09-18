"use client";

import React, { useState } from "react";
import Link from "next/link";
import { ArrowRight, Bot, ChevronDown, CheckCircle2, Search, ExternalLink, Play } from "lucide-react";
import { motion } from "framer-motion";

export default function Home() {
  return (
    <div className="min-h-screen bg-white font-sans text-slate-900 selection:bg-slate-200">
      
      {/* 1. STICKY ALERT BAR */}
      <div className="bg-black text-white text-[11px] font-medium tracking-wide py-2.5 px-4 text-center flex items-center justify-center gap-2 group cursor-pointer w-full z-50 relative">
        Kazi AI is a founding data partner for enterprise ERP connectivity
        <span className="text-zinc-400 group-hover:text-white transition-colors flex items-center">
          Read announcement <ArrowRight size={12} className="ml-1" />
        </span>
      </div>

      {/* 2. FLOATING MENU HEADER */}
      <div className="sticky top-4 z-40 flex justify-center w-full px-4 mt-4 pointer-events-none">
        <header className="pointer-events-auto flex items-center justify-between bg-white/90 backdrop-blur-md border border-slate-200 shadow-sm rounded-[8px] px-4 py-2 w-full max-w-4xl">
          <div className="flex items-center">
            <Link href="/" className="flex items-center gap-2 px-2">
              <Bot size={20} className="text-slate-900" />
              <span className="font-bold text-sm tracking-tight hidden sm:block uppercase">Kazi AI</span>
            </Link>
          </div>
          
          <nav className="hidden md:flex items-center gap-1 text-[13px] font-medium text-slate-600">
            <button className="flex items-center gap-1 hover:text-slate-900 hover:bg-slate-50 px-3 py-1.5 rounded-[8px] transition-colors">
              Product <ChevronDown size={14} className="text-slate-400" />
            </button>
            <button className="flex items-center gap-1 hover:text-slate-900 hover:bg-slate-50 px-3 py-1.5 rounded-[8px] transition-colors">
              Use cases <ChevronDown size={14} className="text-slate-400" />
            </button>
            <button className="flex items-center gap-1 hover:text-slate-900 hover:bg-slate-50 px-3 py-1.5 rounded-[8px] transition-colors">
              Discover <ChevronDown size={14} className="text-slate-400" />
            </button>
            <Link href="#pricing" className="hover:text-slate-900 hover:bg-slate-50 px-3 py-1.5 rounded-[8px] transition-colors">
              Pricing
            </Link>
          </nav>
          
          <div className="flex items-center gap-2">
            <Link href="/auth/login" className="text-[13px] font-medium text-slate-600 hover:text-slate-900 px-3 py-1.5">
              Log in
            </Link>
            <Link href="/auth/register" className="px-4 py-2 text-[13px] font-semibold rounded-[8px] bg-slate-900 text-white hover:bg-slate-800 transition-colors shadow-sm">
              Book demo
            </Link>
          </div>
        </header>
      </div>

      <main className="w-full flex flex-col items-center pt-8">
        
        {/* 3. HERO SECTION TOP (TEXT BLOCK) */}
        <section className="w-full max-w-[1280px] px-6 md:px-12 pt-16 pb-12 flex flex-col md:flex-row items-end justify-between gap-12">
          {/* Headline */}
          <div className="w-full md:w-[60%]">
            <h1 className="text-5xl md:text-[72px] font-medium tracking-tight text-slate-900 leading-[1.05]">
              AI infrastructure for<br />enterprise operations
            </h1>
          </div>
          
          {/* Subtitle & CTA */}
          <div className="w-full md:w-[35%] flex flex-col items-start pb-2">
            <p className="text-[17px] text-slate-700 leading-relaxed font-medium mb-6 max-w-sm">
              Everything executed and tracked in your ERPs, structured for AI and orchestrated in real-time.
            </p>
            <Link href="/auth/register" className="px-6 py-3 rounded-[8px] bg-slate-900 text-white font-medium text-sm hover:bg-slate-800 transition-colors shadow-sm">
              Book demo
            </Link>
          </div>
        </section>

        {/* 4. HERO MEDIA FRAME (MARBLE CANVAS + APP) */}
        <section className="w-full px-4 md:px-8 max-w-[1400px] mb-24">
          <div className="relative w-full rounded-[8px] overflow-hidden bg-slate-100 flex items-center justify-center p-4 md:p-16 border border-slate-200">
            {/* Asset Background Injection */}
            <div className="absolute inset-0 z-0 bg-cover bg-center opacity-60 mix-blend-multiply" 
                 style={{ backgroundImage: 'url("/assets/martin-woortman-fjIwE1i6YXQ-unsplash.jpg")'}}></div>
            
            {/* The 3-Pane App UI Mockup */}
            <div className="relative z-10 w-full max-w-6xl aspect-[16/10] bg-white rounded-[8px] shadow-[0_25px_50px_-12px_rgba(0,0,0,0.3)] border border-slate-200 flex flex-col overflow-hidden">
              {/* App Header */}
              <div className="h-14 border-b border-slate-200 flex items-center px-4 justify-between bg-white">
                <div className="flex items-center gap-6">
                  <div className="flex gap-1.5 opacity-40">
                    <div className="w-3 h-3 rounded-[8px] bg-slate-300"></div>
                    <div className="w-3 h-3 rounded-[8px] bg-slate-300"></div>
                    <div className="w-3 h-3 rounded-[8px] bg-slate-300"></div>
                  </div>
                  <div className="font-bold tracking-tight text-sm flex items-center gap-2 uppercase">
                    <Bot size={16} /> Kazi AI
                  </div>
                </div>
                <div className="flex gap-4 text-xs font-medium text-slate-400">
                  <div className="flex items-center gap-1.5"><Search size={14}/> Search</div>
                </div>
              </div>

              {/* App Body - 3 Columns */}
              <div className="flex-1 flex overflow-hidden bg-white">
                {/* Left slim nav */}
                <div className="w-56 border-r border-slate-200 p-3 hidden md:flex flex-col gap-1">
                  <div className="text-[10px] font-semibold text-slate-400 uppercase tracking-wider px-3 py-2">Workspace</div>
                  <div className="px-3 py-2 text-sm font-medium text-slate-700 bg-slate-50 rounded-[8px] flex items-center gap-2 cursor-pointer"><Bot size={16}/> Copilot</div>
                  <div className="px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 rounded-[8px] flex items-center gap-2 cursor-pointer"><Search size={16}/> Data Explorer</div>
                  <div className="px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-50 rounded-[8px] flex items-center gap-2 cursor-pointer"><CheckCircle2 size={16}/> Approvals</div>
                </div>

                {/* Middle Stream */}
                <div className="w-72 border-r border-slate-200 p-0 hidden lg:flex flex-col">
                  <div className="h-12 border-b border-slate-200 flex items-center px-4 text-xs font-semibold text-slate-800">
                    Activity Feed
                  </div>
                  <div className="flex-1 overflow-auto bg-slate-50 p-3 space-y-3">
                    {[1,2,3].map(i => (
                      <div key={i} className="bg-white p-3 rounded-[8px] border border-slate-200 shadow-sm">
                        <div className="flex items-center gap-2 text-xs font-semibold text-slate-800 mb-1">
                          <div className="w-4 h-4 rounded-[8px] bg-blue-100 text-blue-600 flex items-center justify-center text-[8px]">O</div>
                          Odoo Sync Complete
                        </div>
                        <div className="text-[11px] text-slate-500">200 new invoices synced.</div>
                        <div className="text-[9px] text-slate-400 mt-2 font-mono">2h ago</div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Main Content Pane */}
                <div className="flex-1 flex flex-col bg-white">
                  <div className="h-12 border-b border-slate-200 flex items-center px-6 text-sm font-medium text-slate-800">
                    <Bot size={16} className="mr-2 text-slate-400"/> Chat
                  </div>
                  <div className="flex-1 p-6 md:p-8 overflow-auto flex flex-col">
                    <div className="text-sm font-medium text-slate-500 mb-6">You've requested forward-looking statements from the recent inventory audits.</div>
                    
                    <div className="bg-white border text-left border-slate-200 rounded-[8px] overflow-hidden shadow-sm">
                      <table className="w-full text-xs box-border">
                        <thead className="bg-slate-50 border-b border-slate-200 text-slate-500 font-medium">
                          <tr>
                            <td className="p-3 border-r border-slate-200">Topic</td>
                            <td className="p-3 w-1/2 border-r border-slate-200">Insight</td>
                            <td className="p-3 text-right">Source</td>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-200">
                          <tr>
                            <td className="p-3 align-top font-medium text-slate-800 border-r border-slate-200">Q4 Restock Impact</td>
                            <td className="p-3 align-top text-slate-600 leading-relaxed border-r border-slate-200">
                              Additional ordering actions necessary in Q4 to preempt tariff shortages. Revised models predict 5% saving on bulk transfers.
                            </td>
                            <td className="p-3 align-top text-right text-slate-400">
                              Odoo PO-2933 <sup className="inline-flex items-center justify-center w-3.5 h-3.5 rounded-[8px] bg-emerald-100 text-emerald-700 font-mono text-[9px] cursor-pointer ml-1 relative group">1
                                {/* Simulated Citation Popover */}
                                <div className="absolute top-6 right-0 w-64 bg-white rounded-[8px] shadow-xl border border-slate-200 p-4 text-left hidden group-hover:block z-50">
                                  <div className="flex items-center gap-2 text-slate-900 font-semibold mb-2 text-sm">
                                    <img src="https://upload.wikimedia.org/wikipedia/commons/4/41/Odoo_logo.svg" className="h-3" alt="Odoo"/>
                                    Sarah J., Supply Chain
                                  </div>
                                  <p className="text-slate-600 font-normal leading-relaxed text-xs">
                                    "Yes. In the fourth quarter, we are expecting that we'll see a drop in our gross margin, which is consistent with historical performance... we are actively pulling forward restock schedules."
                                  </p>
                                  <div className="mt-3 pt-3 border-t border-slate-200 text-[10px] text-emerald-600 font-medium flex items-center gap-1">
                                    <ExternalLink size={12}/> View in Odoo ERP
                                  </div>
                                </div>
                              </sup>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>

                    <div className="mt-8">
                      <div className="flex items-center border border-slate-200 rounded-[8px] bg-white px-4 py-2 shadow-sm focus-within:ring-2 focus-within:ring-slate-300">
                        <input className="flex-1 bg-transparent text-sm focus:outline-none placeholder:text-slate-400" placeholder="Ask anything..." />
                        <button className="w-8 h-8 rounded-[8px] bg-slate-100 flex items-center justify-center text-slate-600 hover:bg-slate-200"><ArrowRight size={14}/></button>
                      </div>
                    </div>
                  </div>

                  {/* Audio/Status player bar at bottom */}
                  <div className="h-12 bg-slate-900 text-white flex items-center px-4 shrink-0 m-0 text-xs font-medium justify-between border-t border-slate-800">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 bg-slate-800 rounded-[8px] flex items-center justify-center uppercase font-bold text-[10px] border border-slate-700">ERP</div>
                      <span>Live Sync: Processing PO-2933</span>
                    </div>
                    <div className="flex items-center gap-4 text-slate-400">
                      <span className="font-mono text-[10px]">01:32:17</span>
                      <div className="w-32 h-1 bg-slate-700 rounded-[8px] overflow-hidden hidden md:block">
                        <div className="w-1/3 h-full bg-emerald-400 rounded-[8px]"></div>
                      </div>
                      <span className="font-mono text-[10px]">-2:40:19</span>
                      <Play size={12} className="text-white ml-2"/>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* 5. CLIENT LOGO MARQUEE */}
        <section className="w-full max-w-[1200px] px-6 mb-24 border-b border-t border-slate-100 py-10">
          <div className="flex flex-wrap justify-between items-center gap-8 opacity-60 grayscale filter">
            <div className="text-xl font-serif tracking-widest font-bold">STIFEL</div>
            <div className="text-xl font-black italic tracking-tighter">yahoo!<span className="text-sm font-sans block leading-none text-[#7b0099]">finance</span></div>
            <div className="text-xl font-semibold tracking-tight">OpenAI</div>
            <div className="flex items-center gap-2 font-bold"><Bot size={20}/> Kazi <span className="font-normal text-xs text-slate-500">internal</span></div>
            <div className="text-lg font-bold">Wood<br/><span className="text-sm">Mackenzie</span></div>
            <div className="text-lg font-serif">Janus Henderson</div>
            <div className="text-xl font-medium tracking-tight">MORNINGSTAR</div>
          </div>
        </section>

        {/* 6. TWO OFFERINGS. ONE ENGINE. (BENTO) */}
        <section className="w-full max-w-[1200px] px-6 mb-32">
          <div className="mb-12">
            <h2 className="text-3xl md:text-5xl font-semibold tracking-tight text-slate-900 mb-4 max-w-lg leading-tight">
              Two offerings.<br/>One proprietary data engine.
            </h2>
            <p className="text-slate-600 text-lg max-w-sm">
              Built for operations professionals, supply platforms, and enterprise AI.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 w-full">
            {/* Card 1 */}
            <div className="group cursor-pointer">
              <div className="w-full aspect-[4/5] md:aspect-square bg-slate-100 rounded-[8px] overflow-hidden mb-6 relative border border-slate-200">
                 <div className="absolute inset-0 bg-cover bg-center opacity-80" style={{ backgroundImage: 'url("/assets/engin-akyurt-K6F9-ZteJLA-unsplash.jpg")'}}></div>
                 <div className="absolute inset-0 bg-black/10"></div>
                 <div className="absolute inset-0 flex items-center justify-center p-8">
                   <div className="bg-white/95 backdrop-blur-md p-6 rounded-[8px] shadow-xl w-full max-w-xs border border-white/40 transform transition-transform group-hover:scale-105 duration-300">
                     <div className="w-full h-2 bg-slate-200 mb-3 rounded-[8px]"><div className="h-full w-1/3 bg-slate-900 rounded-[8px]"></div></div>
                     <div className="w-2/3 h-2 bg-slate-200 mb-6 rounded-[8px]"></div>
                     <div className="w-full aspect-video bg-slate-50 border border-slate-200 rounded-[8px] bg-[url('/assets/tom-barrett-2t60-JYQ5pk-unsplash.jpg')] bg-cover opacity-80"></div>
                   </div>
                 </div>
              </div>
              <h3 className="text-xl font-semibold text-slate-900 mb-2">Kazi AI Pro</h3>
              <p className="text-slate-600 text-[15px]">AI for company operations</p>
            </div>

            {/* Card 2 */}
            <div className="group cursor-pointer">
              <div className="w-full aspect-[4/5] md:aspect-square bg-slate-800 rounded-[8px] overflow-hidden mb-6 relative border border-slate-800">
                <div className="absolute inset-0 bg-cover bg-center mix-blend-overlay opacity-60" style={{ backgroundImage: 'url("/assets/engin-akyurt-gJILnne_HFg-unsplash.jpg")'}}></div>
                <div className="absolute inset-0 bg-slate-900/40"></div>
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="bg-slate-900/60 backdrop-blur-md p-6 rounded-[8px] border border-slate-700 text-white font-mono text-[11px] leading-relaxed shadow-2xl transform transition-transform group-hover:scale-105 duration-300 max-w-xs w-full">
                    <span className="text-emerald-400">"eventId"</span>: 221747,<br/>
                    <span className="text-emerald-400">"eventTitle"</span>: <span className="text-green-300">"Q4 Operations Sync"</span>,<br/>
                    <span className="text-emerald-400">"eventDate"</span>: <span className="text-green-300">"2026-08-28T09:30:00"</span>,<br/>
                    <span className="text-emerald-400">"company"</span>: {'{'}<br/>
                    &nbsp;&nbsp;<span className="text-emerald-400">"companyId"</span>: 16178,<br/>
                    &nbsp;&nbsp;<span className="text-emerald-400">"companyName"</span>: <span className="text-green-300">"Kazi AI Core"</span><br/>
                    {'}'}
                  </div>
                </div>
              </div>
              <h3 className="text-xl font-semibold text-slate-900 mb-2">Kazi AI API</h3>
              <p className="text-slate-600 text-[15px]">The world's leading ERP data layer</p>
            </div>
          </div>
        </section>

        {/* 7. 4-CARD CAPABILITY */}
        <section className="w-full max-w-[1200px] px-6 mb-32 border-t border-slate-100 pt-24">
          <div className="flex flex-col md:flex-row md:items-end justify-between mb-16 gap-6">
            <div>
              <h2 className="text-3xl font-medium tracking-tight text-slate-900 mb-4 max-w-lg leading-tight">
                The leading first-party data layer<br/>for institutional finance and AI
              </h2>
              <p className="text-slate-600">For high-performance teams looking to make better decisions faster.</p>
            </div>
            <Link href="#" className="text-sm font-medium text-slate-900 hover:text-slate-600 flex items-center gap-1 group">
              Learn more <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
            </Link>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {/* 1 */}
            <div className="flex flex-col">
              <div className="aspect-[4/3] bg-slate-100 rounded-[8px] mb-5 flex items-center justify-center overflow-hidden border border-slate-200 p-6 relative">
                 <div className="absolute inset-0 bg-cover bg-center opacity-70 mix-blend-overlay" style={{backgroundImage: 'url("/assets/martin-woortman-fjIwE1i6YXQ-unsplash.jpg")'}}></div>
                 <div className="bg-white/90 backdrop-blur border border-slate-200 p-4 rounded-[8px] text-xs text-slate-700 shadow-sm relative z-10 w-full font-medium">
                   "Identify all meaningful stock adjustments from Nairobi, Mombasa, and Kisumu branches in recent inventory reports."
                   <div className="flex gap-2 mt-3 pt-3 border-t border-slate-200 text-slate-400">
                     <Search size={14}/> <ArrowRight size={14}/>
                   </div>
                 </div>
              </div>
              <h4 className="font-semibold text-slate-900 mb-2">Cut execution time</h4>
              <p className="text-sm text-slate-600 leading-relaxed">AI across all first-party ERP material — live transactions, POs, ledgers, fillings.</p>
            </div>
            {/* 2 */}
            <div className="flex flex-col">
              <div className="aspect-[4/3] bg-slate-100 rounded-[8px] mb-5 flex items-center justify-center overflow-hidden border border-slate-200 relative">
                 <div className="absolute inset-0 bg-cover bg-center opacity-60 mix-blend-overlay" style={{backgroundImage: 'url("/assets/tom-barrett-2t60-JYQ5pk-unsplash.jpg")'}}></div>
                 <div className="bg-white px-4 py-2 border shadow-sm rounded-[8px] text-xs font-semibold flex items-center gap-3 w-56 justify-between relative z-10">
                   <div className="w-4 h-4 bg-slate-800 rounded-[8px]"></div>
                   <div className="h-0.5 bg-slate-200 flex-1 relative"><div className="absolute left-0 top-0 h-full bg-slate-400 w-1/3"></div></div>
                   <div className="text-[10px] text-slate-400 font-mono">7:24</div>
                 </div>
              </div>
              <h4 className="font-semibold text-slate-900 mb-2">Be first</h4>
              <p className="text-sm text-slate-600 leading-relaxed">Global live coverage of earnings, stock transfers, M-Pesa receipts, and more.</p>
            </div>
            {/* 3 */}
            <div className="flex flex-col">
              <div className="aspect-[4/3] bg-slate-100 rounded-[8px] mb-5 flex items-center justify-center overflow-hidden border border-slate-200 relative p-6">
                 <div className="absolute inset-0 bg-cover bg-center opacity-60 mix-blend-overlay" style={{backgroundImage: 'url("/assets/engin-akyurt-gJILnne_HFg-unsplash.jpg")'}}></div>
                 <div className="bg-white rounded-[8px] shadow-sm border border-slate-200 p-4 w-full transform rotate-1 relative z-10">
                   <div className="flex items-center gap-2 mb-2">
                     <div className="w-6 h-6 rounded-[8px] bg-slate-200"></div>
                     <span className="text-[10px] font-bold">CFO Approval</span>
                   </div>
                   <p className="text-xs text-slate-600 font-medium">"Expenditure growth was broad-based with our structured automation protocol..."</p>
                   <div className="absolute -bottom-2 -right-2 bg-slate-900 text-white rounded-[8px] p-1.5 shadow-md border border-slate-700">
                     <CheckCircle2 size={12}/>
                   </div>
                 </div>
              </div>
              <h4 className="font-semibold text-slate-900 mb-2">Trust every finding</h4>
              <p className="text-sm text-slate-600 leading-relaxed">50M+ first-party transactions. Every output traceable directly to its ERP source record.</p>
            </div>
            {/* 4 */}
            <div className="flex flex-col">
              <div className="aspect-[4/3] bg-white rounded-[8px] mb-5 flex items-center justify-center overflow-hidden border border-slate-200 p-4 relative">
                 <div className="absolute inset-0 bg-cover bg-center opacity-20" style={{backgroundImage: 'url("/assets/augustine-wong-zlSGWreXHGU-unsplash.jpg")'}}></div>
                 <div className="w-full h-full flex flex-col gap-2 relative z-10 bg-white/50 p-2 border border-slate-100 backdrop-blur-sm">
                   {[1,2,3].map(i => (
                     <div key={i} className="flex-1 border rounded-[8px] border-slate-200 bg-white/80 p-2 flex items-end gap-1">
                       <div className="w-1/6 bg-slate-800 rounded-[8px] h-full border border-slate-900"></div>
                       <div className="w-1/6 bg-slate-300 rounded-[8px] h-[60%] border border-slate-400"></div>
                       <div className="w-1/6 bg-slate-800 rounded-[8px] h-[80%] border border-slate-900"></div>
                       <div className="w-1/6 bg-slate-300 rounded-[8px] h-[40%] border border-slate-400"></div>
                       <div className="flex-1 flex flex-col h-full justify-start items-end"><span className="text-[8px] font-mono text-slate-500">Q{i} Vol.</span></div>
                     </div>
                   ))}
                 </div>
              </div>
              <h4 className="font-semibold text-slate-900 mb-2">Spot inflection points</h4>
              <p className="text-sm text-slate-600 leading-relaxed">Identify changes in purchasing patterns, KPIs, and strategic focus over time.</p>
            </div>
          </div>
        </section>

        {/* 8. CLAUDE / CHATGPT MCP HIGHLIGHT SECTION */}
        <section className="w-full max-w-[1200px] px-6 mb-32 flex flex-col md:flex-row gap-12 pt-24 border-t border-slate-100">
          <div className="w-full md:w-1/3">
            <h2 className="text-2xl font-semibold tracking-tight text-slate-900 mb-4 max-w-[200px] leading-tight">
              Kazi AI MCP connects your AI environment to 16K+ operational data points
            </h2>
          </div>
          <div className="w-full md:w-2/3">
            <div className="w-full aspect-[21/9] bg-slate-100 rounded-[8px] overflow-hidden mb-6 relative border border-slate-200">
              <div className="absolute inset-0 bg-cover bg-center mix-blend-multiply opacity-50" style={{backgroundImage: 'url("/assets/martin-woortman-fjIwE1i6YXQ-unsplash.jpg")'}}></div>
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="bg-white/90 backdrop-blur-md p-3 rounded-[8px] shadow-sm flex gap-4 border border-slate-200">
                  <div className="w-12 h-12 rounded-[8px] bg-[#D97757] flex items-center justify-center border border-[#b35b40]">
                    <svg className="w-7 h-7 text-white" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C12 2 12.5 7.5 17 8C12.5 8.5 12 14 12 14C12 14 11.5 8.5 7 8C11.5 7.5 12 2 12 2Z"/><path d="M19 14C19 14 19.3 17.1 22 17.5C19.3 17.9 19 21 19 21C19 21 18.7 17.9 16 17.5C18.7 17.1 19 14 19 14Z"/><path d="M6 16C6 16 6.3 19.1 9 19.5C6.3 19.9 6 23 6 23C6 23 5.7 19.9 3 19.5C5.7 19.1 6 16 6 16Z"/></svg>
                  </div>
                  <div className="w-12 h-12 rounded-[8px] bg-slate-900 flex items-center justify-center border border-slate-950">
                    <Bot className="w-7 h-7 text-white"/>
                  </div>
                  <div className="w-12 h-12 rounded-[8px] bg-teal-600 flex items-center justify-center border border-teal-700">
                    <svg className="w-7 h-7 text-white" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C17.52 2 22 6.48 22 12C22 17.52 17.52 22 12 22C6.48 22 2 17.52 2 12C2 6.48 6.48 2 12 2ZM12 4C7.58 4 4 7.58 4 12C4 16.42 7.58 20 12 20C16.42 20 20 16.42 20 12C20 7.58 16.42 4 12 4ZM12 8C14.21 8 16 9.79 16 12C16 14.21 14.21 16 12 16C9.79 16 8 14.21 8 12C8 9.79 9.79 8 12 8ZM12 10C10.9 10 10 10.9 10 12C10 13.1 10.9 14 12 14C13.1 14 14 13.1 14 12C14 10.9 13.1 10 12 10Z"/></svg>
                  </div>
                </div>
              </div>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
              <p className="text-sm text-slate-600 leading-relaxed font-medium">
                Pull live ERP tables, PO PDFs, and inventory syncs into Claude or ChatGPT via MCP. Combine our global ERP adapter with any other dataset. Run agentic research securely.
              </p>
              <div className="text-sm text-slate-600 leading-relaxed font-medium flex flex-col">
                <span className="mb-4">Included with Kazi AI Pro at no extra cost.</span>
                <Link href="#" className="text-slate-900 hover:text-slate-600 flex items-center gap-1 group font-semibold">
                  Learn more <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
                </Link>
              </div>
            </div>
          </div>
        </section>

      </main>

      {/* 9. FOOTER (Quartr styling) */}
      <footer className="w-full bg-white border-t border-slate-100 pt-16 pb-12">
        <div className="max-w-[1200px] mx-auto px-6 grid grid-cols-2 lg:grid-cols-6 gap-8">
          <div className="col-span-2">
            <Link href="/" className="flex items-center gap-2 mb-6">
              <Bot size={24} className="text-slate-900" />
              <span className="font-bold text-lg tracking-tight uppercase">Kazi AI</span>
            </Link>
            <p className="text-xs text-slate-500 font-medium">© {new Date().getFullYear()} Kazi AI. All rights reserved.</p>
          </div>
          
          <div>
            <h4 className="font-semibold text-slate-900 mb-4 text-sm">Product</h4>
            <ul className="space-y-3 text-sm text-slate-600 font-medium">
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Pro</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">API</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">MCP Integration</Link></li>
            </ul>
          </div>
          
          <div>
            <h4 className="font-semibold text-slate-900 mb-4 text-sm">Use cases</h4>
            <ul className="space-y-3 text-sm text-slate-600 font-medium">
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Asset managers</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Investment banks</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Corporations</Link></li>
            </ul>
          </div>
          
          <div>
            <h4 className="font-semibold text-slate-900 mb-4 text-sm">Discover</h4>
            <ul className="space-y-3 text-sm text-slate-600 font-medium">
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Enterprise Search</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Reconciliation</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Inventory Automation</Link></li>
            </ul>
          </div>
          
          <div>
            <h4 className="font-semibold text-slate-900 mb-4 text-sm">Company</h4>
            <ul className="space-y-3 text-sm text-slate-600 font-medium">
              <li><Link href="#" className="hover:text-slate-900 transition-colors">About us</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Newsroom</Link></li>
              <li><Link href="#" className="hover:text-slate-900 transition-colors">Careers</Link></li>
            </ul>
          </div>
        </div>
      </footer>
    </div>
  );
}
