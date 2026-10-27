"use client";

import React, { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Bot, Loader2 } from "lucide-react";
import { useAuth } from "@/lib/auth";

export default function RegisterPage() {
  const router = useRouter();
  const { user, register } = useAuth();
  const [form, setForm] = useState({ name: "", email: "", password: "", confirm: "" });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (user) router.replace("/app");
  }, [user, router]);

  const update = (key: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((prev) => ({ ...prev, [key]: e.target.value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    if (form.password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (form.password !== form.confirm) {
      setError("Passwords do not match.");
      return;
    }
    setLoading(true);
    try {
      await register(form.name.trim(), form.email.trim(), form.password);
      router.push("/app");
    } catch (err: any) {
      setError(err.message || "Unable to create account");
    } finally {
      setLoading(false);
    }
  };

  const input =
    "w-full px-3 py-2.5 bg-transparent border border-white/10 rounded-md text-sm text-white focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/20 transition-all placeholder:text-white/20";

  return (
    <div className="min-h-screen bg-[#111111] flex flex-col items-center justify-center px-4 py-10 font-sans selection:bg-slate-700">
      <Link href="/" className="flex items-center gap-2 mb-8">
        <Bot size={20} className="text-white opacity-90" />
        <span className="font-bold text-sm tracking-[0.1em] text-white/90 uppercase pr-1">Kazi AI</span>
      </Link>

      <div className="bg-[#18181A] border border-white/5 w-full max-w-[420px] rounded-[16px] p-8 shadow-2xl">
        <h1 className="text-xl font-semibold text-white mb-1">Create your account</h1>
        <p className="mb-6 text-xs text-white/40">Your workspace gets its own private tenant and chat history.</p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <label className="text-xs font-medium text-white/70">Full name</label>
            <input type="text" autoComplete="name" placeholder="Jane Doe" required value={form.name} onChange={update("name")} className={input} />
          </div>
          <div className="space-y-2">
            <label className="text-xs font-medium text-white/70">Work email</label>
            <input type="email" autoComplete="email" placeholder="you@company.com" required value={form.email} onChange={update("email")} className={input} />
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <label className="text-xs font-medium text-white/70">Password</label>
              <input type="password" autoComplete="new-password" placeholder="8+ characters" required value={form.password} onChange={update("password")} className={input} />
            </div>
            <div className="space-y-2">
              <label className="text-xs font-medium text-white/70">Confirm</label>
              <input type="password" autoComplete="new-password" placeholder="Repeat password" required value={form.confirm} onChange={update("confirm")} className={input} />
            </div>
          </div>

          {error && <p className="text-xs text-rose-400">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 bg-white text-black font-semibold text-sm rounded-md hover:bg-white/90 transition-colors flex justify-center items-center mt-2 disabled:opacity-70 disabled:cursor-not-allowed"
          >
            {loading ? <Loader2 size={16} className="animate-spin text-black" /> : "Create account"}
          </button>
        </form>

        <p className="mt-6 text-center text-[13px] text-white/50">
          Already have an account?{" "}
          <Link href="/auth/login" className="text-white/80 underline decoration-white/20 underline-offset-2 hover:text-white">
            Sign in
          </Link>
        </p>
      </div>
    </div>
  );
}
