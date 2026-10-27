"use client";

import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { apiGet, apiPatch, apiPost } from "./api";

export type AuthUser = {
  id: string;
  tenant_id: string;
  email: string;
  name: string;
  created_at?: string;
  updated_at?: string;
};

type AuthContextValue = {
  user: AuthUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  updateProfile: (name: string, email: string) => Promise<void>;
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

type AuthResponse = { status: string; user: AuthUser };

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const res = await apiGet<AuthResponse>("/auth/me");
      setUser(res.user);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const login = useCallback(async (email: string, password: string) => {
    const res = await apiPost<AuthResponse>("/auth/login", { email, password });
    setUser(res.user);
  }, []);

  const register = useCallback(async (name: string, email: string, password: string) => {
    const res = await apiPost<AuthResponse>("/auth/register", { name, email, password });
    setUser(res.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiPost("/auth/logout");
    } finally {
      setUser(null);
      if (typeof window !== "undefined") {
        window.localStorage.removeItem("kazi_conv_id");
      }
    }
  }, []);

  const updateProfile = useCallback(async (name: string, email: string) => {
    const res = await apiPatch<AuthResponse>("/auth/profile", { name, email });
    setUser(res.user);
  }, []);

  const changePassword = useCallback(async (currentPassword: string, newPassword: string) => {
    await apiPost("/auth/password", { current_password: currentPassword, new_password: newPassword });
  }, []);

  const value = useMemo(
    () => ({ user, loading, login, register, logout, updateProfile, changePassword, refresh }),
    [user, loading, login, register, logout, updateProfile, changePassword, refresh]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}
