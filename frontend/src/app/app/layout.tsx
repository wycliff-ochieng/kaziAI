import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Kazi AI — Workspace",
  description: "AI operations workspace for enterprise ERPs.",
};

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <div className="h-screen w-full overflow-hidden">{children}</div>;
}
