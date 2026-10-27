"use client";

import React, { useEffect, useState } from "react";
import { CheckCircle2, KeyRound, LogOut, UserRound } from "lucide-react";
import { useAuth } from "@/lib/auth";

export function ProfileView() {
  const { user, updateProfile, changePassword, logout } = useAuth();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [profileMsg, setProfileMsg] = useState<{ tone: "ok" | "error"; text: string } | null>(null);
  const [savingProfile, setSavingProfile] = useState(false);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordMsg, setPasswordMsg] = useState<{ tone: "ok" | "error"; text: string } | null>(null);
  const [savingPassword, setSavingPassword] = useState(false);

  useEffect(() => {
    if (user) {
      setName(user.name);
      setEmail(user.email);
    }
  }, [user]);

  if (!user) {
    return <div className="flex-1 p-8 text-sm text-slate-400">Loading account…</div>;
  }

  const input =
    "w-full rounded-lg border border-slate-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-slate-200";

  const onSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingProfile(true);
    setProfileMsg(null);
    try {
      await updateProfile(name.trim(), email.trim());
      setProfileMsg({ tone: "ok", text: "Profile updated." });
    } catch (err: any) {
      setProfileMsg({ tone: "error", text: err.message || "Could not update profile" });
    } finally {
      setSavingProfile(false);
    }
  };

  const onChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordMsg(null);
    if (newPassword.length < 8) {
      setPasswordMsg({ tone: "error", text: "New password must be at least 8 characters." });
      return;
    }
    if (newPassword !== confirmPassword) {
      setPasswordMsg({ tone: "error", text: "New passwords do not match." });
      return;
    }
    setSavingPassword(true);
    try {
      await changePassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setPasswordMsg({ tone: "ok", text: "Password changed." });
    } catch (err: any) {
      setPasswordMsg({ tone: "error", text: err.message || "Could not change password" });
    } finally {
      setSavingPassword(false);
    }
  };

  return (
    <div className="flex-1 overflow-y-auto p-8">
      <div className="mb-6 max-w-2xl">
        <h2 className="text-lg font-semibold tracking-tight text-slate-900">Profile</h2>
        <p className="text-sm text-slate-500">Manage the account used to access this workspace.</p>
      </div>

      <div className="mb-6 max-w-2xl rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-slate-900 text-sm font-bold text-white">
            {(user.name || user.email).slice(0, 1).toUpperCase()}
          </div>
          <div>
            <p className="text-sm font-semibold text-slate-800">{user.name}</p>
            <p className="font-mono text-[11px] text-slate-400">{user.tenant_id}</p>
          </div>
        </div>

        <form onSubmit={onSaveProfile} className="space-y-3">
          <div className="space-y-1">
            <label className="text-xs font-medium text-slate-600">Full name</label>
            <input className={input} value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="space-y-1">
            <label className="text-xs font-medium text-slate-600">Email</label>
            <input className={input} type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          {profileMsg && (
            <p className={`flex items-center gap-1 text-xs ${profileMsg.tone === "ok" ? "text-emerald-700" : "text-rose-600"}`}>
              {profileMsg.tone === "ok" && <CheckCircle2 size={12} />} {profileMsg.text}
            </p>
          )}
          <div className="flex items-center gap-2 pt-1">
            <button
              type="submit"
              disabled={savingProfile}
              className="rounded-lg bg-slate-900 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
            >
              {savingProfile ? "Saving…" : "Save profile"}
            </button>
            <button
              type="button"
              onClick={() => logout()}
              className="flex items-center gap-1 rounded-lg border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50"
            >
              <LogOut size={12} /> Sign out
            </button>
          </div>
        </form>
      </div>

      <div className="max-w-2xl rounded-xl border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-3 flex items-center gap-2">
          <KeyRound size={15} className="text-slate-400" />
          <h3 className="text-sm font-semibold tracking-tight text-slate-800">Change password</h3>
        </div>
        <form onSubmit={onChangePassword} className="space-y-3">
          <div className="space-y-1">
            <label className="text-xs font-medium text-slate-600">Current password</label>
            <input className={input} type="password" autoComplete="current-password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} required />
          </div>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="space-y-1">
              <label className="text-xs font-medium text-slate-600">New password</label>
              <input className={input} type="password" autoComplete="new-password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-medium text-slate-600">Confirm new password</label>
              <input className={input} type="password" autoComplete="new-password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required />
            </div>
          </div>
          {passwordMsg && (
            <p className={`flex items-center gap-1 text-xs ${passwordMsg.tone === "ok" ? "text-emerald-700" : "text-rose-600"}`}>
              {passwordMsg.tone === "ok" && <CheckCircle2 size={12} />} {passwordMsg.text}
            </p>
          )}
          <button
            type="submit"
            disabled={savingPassword}
            className="rounded-lg bg-slate-900 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
          >
            {savingPassword ? "Updating…" : "Change password"}
          </button>
        </form>
      </div>
    </div>
  );
}
