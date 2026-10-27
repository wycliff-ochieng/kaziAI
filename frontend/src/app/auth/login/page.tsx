"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Bot, Loader2 } from "lucide-react";
import { useAuth } from "@/lib/auth";

export default function LoginPage() {
  const router = useRouter();
  const { user, login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (user) router.replace("/app");
  }, [user, router]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password) return;
    setLoading(true);
    setError("");
    try {
      await login(email, password);
      router.push("/app");
    } catch (err: any) {
      setError(err.message || "Unable to sign in");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#111111] flex flex-col items-center justify-center px-4 font-sans selection:bg-slate-700">
      <Link href="/" className="flex items-center gap-2 mb-8">
        <Bot size={20} className="text-white opacity-90" />
        <span className="font-bold text-sm tracking-[0.1em] text-white/90 uppercase pr-1">Kazi AI</span>
      </Link>

      <div className="bg-[#18181A] border border-white/5 w-full max-w-[380px] rounded-[16px] p-8 shadow-2xl">
        <h1 className="text-xl font-semibold text-white mb-1">Sign in</h1>
        <p className="mb-6 text-xs text-white/40">Use your Kazi AI account to open the workspace.</p>

        <form onSubmit={handleLogin} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-medium text-white/70">Email</label>
            <input
              type="email"
              autoComplete="email"
              placeholder="you@company.com"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full px-3 py-2.5 bg-transparent border border-white/10 rounded-md text-sm text-white focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/20 transition-all placeholder:text-white/20"
            />
          </div>

          <div className="space-y-2">
            <label className="text-xs font-medium text-white/70">Password</label>
            <input
              type="password"
              autoComplete="current-password"
              placeholder="••••••••"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-3 py-2.5 bg-transparent border border-white/10 rounded-md text-sm text-white focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/20 transition-all placeholder:text-white/20"
            />
          </div>

          {error && <p className="text-xs text-rose-400">{error}</p>}

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
          <span className="flex-shrink-0 mx-4 text-[11px] text-white/40 font-medium">new here?</span>
          <div className="flex-grow border-t border-white/10"></div>
        </div>

        <Link
          href="/auth/register"
          className="flex w-full items-center justify-center rounded-md border border-white/10 py-2.5 text-sm font-medium text-white/80 transition-colors hover:bg-white/5 hover:text-white"
        >
          Create an account
        </Link>
      </div>

      <div className="mt-6 flex flex-col items-center space-y-4">
        <p className="text-[10px] text-white/30">
          Kazi AI&apos;s{" "}
          <Link href="#" className="underline decoration-white/20 hover:text-white/60">Terms of service</Link>{" "}
          and{" "}
          <Link href="#" className="underline decoration-white/20 hover:text-white/60">Privacy policy</Link>{" "}
          apply
        </p>
      </div>
    </div>
  );
}
