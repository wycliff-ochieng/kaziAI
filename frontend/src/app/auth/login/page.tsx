"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Bot, Loader2 } from "lucide-react";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);

  const handleLogin = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;
    
    setLoading(true);
    // Simulate simple authentication latency
    setTimeout(() => {
      // Directs into the workspace (simple authentication implementation)
      router.push("/app");
    }, 1000);
  };

  return (
    <div className="min-h-screen bg-[#111111] flex flex-col items-center justify-center px-4 font-sans selection:bg-slate-700">
      
      {/* Centered Logo above the card */}
      <Link href="/" className="flex items-center gap-2 mb-8">
        <Bot size={20} className="text-white opacity-90" />
        <span className="font-bold text-sm tracking-[0.1em] text-white/90 uppercase pr-1">Kazi AI</span>
      </Link>
      
      {/* Dark Sign-in Card */}
      <div className="bg-[#18181A] border border-white/5 w-full max-w-[380px] rounded-[16px] p-8 shadow-2xl">
        <h1 className="text-xl font-semibold text-white mb-6">Sign in</h1>
        
        <form onSubmit={handleLogin} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-medium text-white/70">Email</label>
            <input 
              type="email" 
              placeholder="your@email.com" 
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-3 py-2.5 bg-transparent border border-white/10 rounded-md text-sm text-white focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/20 transition-all placeholder:text-white/20"
            />
          </div>
          
          <button 
            type="submit" 
            disabled={loading}
            className="w-full py-2.5 bg-white text-black font-semibold text-sm rounded-md hover:bg-white/90 transition-colors flex justify-center items-center mt-2 disabled:opacity-70 disabled:cursor-not-allowed"
          >
            {loading ? <Loader2 size={16} className="animate-spin text-black" /> : "Sign in"}
          </button>
        </form>
        
        <div className="relative flex items-center py-6">
          <div className="flex-grow border-t border-white/10"></div>
          <span className="flex-shrink-0 mx-4 text-[11px] text-white/40 font-medium">or sign in with</span>
          <div className="flex-grow border-t border-white/10"></div>
        </div>
        
        {/* Third Party Login Buttons matching Screenshot */}
        <div className="grid grid-cols-3 gap-3">
          <button className="flex items-center justify-center p-2.5 bg-[#202022] hover:bg-[#2A2A2C] border border-white/5 rounded-md transition-colors">
            {/* X (Twitter) Logo */}
            <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] text-white fill-current" aria-hidden="true"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.005 3.869H5.078z"></path></svg>
          </button>
          <button className="flex items-center justify-center p-2.5 bg-[#202022] hover:bg-[#2A2A2C] border border-white/5 rounded-md transition-colors">
            {/* Google Logo */}
            <svg viewBox="0 0 24 24" className="w-[18px] h-[18px]" aria-hidden="true">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.58c2.08-1.92 3.28-4.74 3.28-8.09z" />
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
            </svg>
          </button>
          <button className="flex items-center justify-center p-2.5 bg-[#202022] hover:bg-[#2A2A2C] border border-white/5 rounded-md transition-colors">
            {/* Apple Logo */}
            <svg viewBox="0 0 24 24" className="w-[18px] h-[18px] text-white fill-current" aria-hidden="true"><path d="M17.062 14.945c.032 1.934 1.7 2.573 1.737 2.585-.02.072-.266.902-.871 1.777-.523.754-1.077 1.498-1.921 1.517-.828.016-1.096-.489-2.046-.489-.953 0-1.258.468-2.046.505-.808.037-1.434-.787-1.96-1.536-1.127-1.584-1.986-4.48-1.405-6.44.286-.968.918-1.602 1.648-1.986.711-.371 1.531-.402 1.834-.402 1.002 0 1.916.685 2.502.685.586 0 1.649-.806 2.819-.685.492.016 1.875.198 2.766 1.496-.07.039-1.648 1.01-1.614 2.973zM15.485 8.163c.48-.567.801-1.353.714-2.139-.672.027-1.511.434-2.008 1.01-.437.512-.816 1.314-.711 2.083.754.059 1.523-.393 2.005-.954z"></path></svg>
          </button>
        </div>
      </div>
      
      {/* Bottom Footer Links */}
      <div className="mt-6 flex flex-col items-center space-y-4">
        <Link href="/auth/register" className="text-[13px] text-white/50 hover:text-white/80 transition-colors">
          Don't have an account? <span className="underline decoration-white/20 underline-offset-2">Get in touch</span>
        </Link>
        <p className="text-[10px] text-white/30">
          Kazi AI's <Link href="#" className="underline decoration-white/20 hover:text-white/60">Terms of service</Link> and <Link href="#" className="underline decoration-white/20 hover:text-white/60">Privacy policy</Link> apply
        </p>
      </div>

    </div>
  );
}
