"use client";

import React from "react";
import Link from "next/link";
import { ArrowRight, Bot, Check } from "lucide-react";

export default function RegisterPage() {
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    alert("Request received! A member of the Kazi AI team will be in touch shortly.");
  };

  return (
    <div className="min-h-screen bg-white font-sans text-slate-900 leading-normal selection:bg-slate-200">
      
      {/* 2. MENU HEADER (Simple standard full width to match image 1) */}
      <header className="flex items-center justify-between bg-white px-6 py-4 w-full border-b border-transparent">
        <div className="flex items-center w-[20%]">
          <Link href="/" className="flex items-center gap-2">
            <Bot size={20} className="text-slate-900" />
            <span className="font-bold text-sm tracking-tight hidden sm:block uppercase">Kazi AI</span>
          </Link>
        </div>
        
        <nav className="hidden md:flex items-center justify-center gap-6 text-[13px] font-medium text-slate-600 flex-1">
          <Link href="#" className="hover:text-slate-900 transition-colors flex items-center gap-1">Product <span className="text-[10px] text-slate-400 font-mono">+</span></Link>
          <Link href="#" className="hover:text-slate-900 transition-colors flex items-center gap-1">Use cases <span className="text-[10px] text-slate-400 font-mono">+</span></Link>
          <Link href="#" className="hover:text-slate-900 transition-colors flex items-center gap-1">Discover <span className="text-[10px] text-slate-400 font-mono">+</span></Link>
          <Link href="#" className="hover:text-slate-900 transition-colors">Pricing</Link>
        </nav>
        
        <div className="flex items-center justify-end w-[20%]">
          <Link href="/auth/login" className="text-[13px] font-medium text-slate-600 hover:text-slate-900 flex items-center gap-1">
            Log in <ArrowRight size={12} />
          </Link>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-16 grid grid-cols-1 md:grid-cols-2 gap-16 md:gap-24">
        
        {/* Left Column (Copy) */}
        <div className="flex flex-col pt-4">
          <h1 className="text-4xl md:text-[44px] font-medium tracking-tight text-slate-900 mb-4 leading-tight">
            Get in touch with sales
          </h1>
          <p className="text-[15px] font-medium text-slate-600 mb-8 max-w-sm leading-relaxed">
            Learn more about Kazi AI Pro or how you can leverage our API.
          </p>

          <ul className="space-y-4 mb-24 max-w-sm">
            <li className="flex items-start gap-4 text-sm font-medium text-slate-700">
              <Check size={16} className="text-slate-400 mt-0.5 shrink-0" strokeWidth={2.5}/>
              Pick a time that works for you
            </li>
            <li className="flex items-start gap-4 text-sm font-medium text-slate-700">
              <Check size={16} className="text-slate-400 mt-0.5 shrink-0" strokeWidth={2.5}/>
              Get a live demo tailored to your needs
            </li>
            <li className="flex items-start gap-4 text-sm font-medium text-slate-700">
              <Check size={16} className="text-slate-400 mt-0.5 shrink-0" strokeWidth={2.5}/>
              Be up and running in no time
            </li>
          </ul>

          {/* Logos Grid matching image 1 */}
          <div className="grid grid-cols-3 gap-6 items-center opacity-70 grayscale">
            <div className="text-sm font-bold leading-tight">Sovereign<br/>Globally</div>
            <div className="text-lg font-serif">Janus Henderson<br/><span className="text-[9px] uppercase tracking-widest text-slate-500">INVESTORS</span></div>
            <div className="text-xl font-bold tracking-tighter uppercase font-serif">FORTUNE</div>
            <div className="text-sm font-bold leading-tight">Knowledge<br/>Globally</div>
            <div className="text-xl font-semibold tracking-tighter">BCG</div>
            <div className="text-lg font-bold italic tracking-tighter">stocktwits</div>
          </div>
        </div>

        {/* Right Column (Form) */}
        <div className="flex flex-col">
          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-2 relative">
              <label className="text-[13px] font-medium text-slate-700">Business email*</label>
              <input type="email" placeholder="name@company.com" required className="w-full px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-sm text-slate-900 focus:outline-none focus:border-slate-500 focus:ring-1 focus:ring-slate-500 shadow-sm" />
            </div>

            <div className="space-y-2 relative">
              <label className="text-[13px] font-medium text-slate-700">Company *</label>
              <input type="text" placeholder="Company name" required className="w-full px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-sm text-slate-900 focus:outline-none focus:border-slate-500 focus:ring-1 focus:ring-slate-500 shadow-sm" />
            </div>

            <div className="space-y-2 relative">
              <label className="text-[13px] font-medium text-slate-700">Name *</label>
              <input type="text" placeholder="First and last name" required className="w-full px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-sm text-slate-900 focus:outline-none focus:border-slate-500 focus:ring-1 focus:ring-slate-500 shadow-sm" />
            </div>

            <div className="space-y-2 relative">
              <label className="text-[13px] font-medium text-slate-700">Phone (incl. country code)</label>
              <input type="tel" placeholder="+1 (123) - 456 78 90" className="w-full px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-sm text-slate-900 focus:outline-none focus:border-slate-500 focus:ring-1 focus:ring-slate-500 shadow-sm" />
            </div>

            <div className="space-y-3 pt-2">
              <label className="text-[13px] font-medium text-slate-700">Which product(s) are you interested in? *</label>
              <div className="flex flex-col gap-2.5">
                <label className="flex items-center gap-3 cursor-pointer group">
                  <input type="checkbox" className="w-4 h-4 rounded border-slate-300 text-slate-900 focus:ring-slate-900" />
                  <span className="text-sm font-medium text-slate-700 group-hover:text-slate-900 transition-colors">Kazi AI Pro & MCP</span>
                </label>
                <label className="flex items-center gap-3 cursor-pointer group">
                  <input type="checkbox" className="w-4 h-4 rounded border-slate-300 text-slate-900 focus:ring-slate-900" />
                  <span className="text-sm font-medium text-slate-700 group-hover:text-slate-900 transition-colors">Kazi AI API</span>
                </label>
              </div>
            </div>

            <div className="space-y-2 relative pt-2">
              <label className="text-[13px] font-medium text-slate-700">Would you like to add anything?</label>
              <textarea rows={4} placeholder="Tell us more about your needs" className="w-full px-3 py-2.5 bg-white border border-slate-300 rounded-lg text-sm text-slate-900 focus:outline-none focus:border-slate-500 focus:ring-1 focus:ring-slate-500 shadow-sm resize-none"></textarea>
            </div>

            <div className="pt-2">
               {/* Note: User request didn't specify exactly, but in image it's sharp cornered full width button */}
               <button type="submit" className="w-full py-3 bg-[#0a0a0b] text-white font-medium text-[13px] rounded-lg hover:bg-black transition-colors flex justify-center items-center">
                 Send
               </button>
            </div>
            
          </form>
        </div>

      </main>
    </div>
  );
}
