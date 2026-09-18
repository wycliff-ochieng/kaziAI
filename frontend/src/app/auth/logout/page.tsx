"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { Bot, LogOut, CheckCircle2 } from "lucide-react";
import { useRouter } from "next/navigation";

export default function LogoutPage() {
  const router = useRouter();

  useEffect(() => {
    // In a real app, clear tokens, user state, etc. here
    const timer = setTimeout(() => {
      router.push("/auth/login");
    }, 3000);
    return () => clearTimeout(timer);
  }, [router]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-zinc-50 px-4">
      <div className="max-w-md w-full bg-white rounded-2xl shadow-xl border border-zinc-100 p-8 text-center space-y-6 transform animate-in fade-in zoom-in-95 duration-500">
        
        <div className="flex justify-center">
          <div className="w-16 h-16 bg-zinc-900 rounded-2xl flex items-center justify-center text-emerald-400 relative shadow-lg shadow-zinc-200">
            <Bot size={32} />
            <div className="absolute -bottom-2 -right-2 w-8 h-8 bg-white rounded-full flex items-center justify-center shadow-sm">
              <CheckCircle2 className="text-emerald-500" size={24} />
            </div>
          </div>
        </div>

        <div>
          <h2 className="text-2xl font-bold tracking-tight text-zinc-900">Successfully logged out</h2>
          <p className="mt-2 text-sm text-zinc-500 font-medium">Your session has been securely ended.</p>
        </div>

        <div className="bg-zinc-50 rounded-xl p-4 flex items-center gap-3 text-sm font-medium text-zinc-600 border border-zinc-100 mb-6">
          <LogOut className="text-zinc-400" size={18} />
          Redirecting you to the login page...
        </div>

        <Link href="/auth/login" className="inline-block w-full py-2.5 px-4 bg-emerald-500 text-white rounded-lg text-sm font-semibold hover:bg-emerald-600 transition-colors shadow-md shadow-emerald-500/20">
          Return to Login
        </Link>
      </div>
    </div>
  );
}
