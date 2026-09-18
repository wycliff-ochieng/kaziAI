"use client";

// Dev/QA fixture page (unlinked): renders every registry block with
// representative props. Open at /app/storybook while running `npm run dev`.
import React from "react";
import { RenderBlock } from "@/components/registry";
import type { UIBlock } from "@/lib/ws";

const FIXTURES: { name: string; block: UIBlock }[] = [
  {
    name: "table — vendors",
    block: {
      type: "table",
      props: {
        title: "Found 2 vendor(s).",
        rows: [
          { id: "20", name: "ABC Ltd", email: "procurement@abc.example.com", city: "Nairobi", balance: 0 },
          { id: "21", name: "Thika Suppliers", email: "sales@thikasuppliers.co.ke", city: "Thika", balance: 12000 },
        ],
      },
    },
  },
  {
    name: "table — POs (chart toggle works when numeric columns exist)",
    block: {
      type: "table",
      props: {
        title: "Retrieved 2 purchase order(s).",
        rows: [
          { id: "501", order_no: "PO00501", vendor_name: "ABC Ltd", amount_total: 2900, state: "purchase" },
          { id: "502", order_no: "PO00502", vendor_name: "Thika Suppliers", amount_total: 1140, state: "draft" },
        ],
      },
    },
  },
  {
    name: "chart — bar",
    block: {
      type: "chart",
      props: {
        kind: "bar",
        labelKey: "vendor_name",
        series: ["amount_total"],
        data: [
          { vendor_name: "ABC Ltd", amount_total: 2900 },
          { vendor_name: "Thika Suppliers", amount_total: 1140 },
        ],
      },
    },
  },
  {
    name: "chart — pie",
    block: {
      type: "chart",
      props: {
        kind: "pie",
        labelKey: "vendor_name",
        series: ["amount_total"],
        data: [
          { vendor_name: "ABC Ltd", amount_total: 2900 },
          { vendor_name: "Thika Suppliers", amount_total: 1140 },
        ],
      },
    },
  },
  {
    name: "dry_run — RFQ (LOW)",
    block: {
      type: "dry_run",
      props: {
        operation: "CREATE_RFQ",
        impact_summary: "Simulated creation of RFQ for ABC Ltd with 1 lines. Subtotal: $2500.00, Tax: $400.00, Total: $2900.00",
        risk_level: "LOW",
        confirmation_prompt: "Do you want to draft RFQ for ABC Ltd (Total: $2900.00)?",
        confirmation_token: "CONFIRM_RFQ_20_fixture",
      },
    },
  },
  {
    name: "dry_run — receipt (HIGH)",
    block: {
      type: "dry_run",
      props: {
        operation: "RECEIVE_PURCHASE_ORDER",
        impact_summary: "Receiving PO PO00501 will validate the full inbound picking and increase inventory stock.",
        risk_level: "HIGH",
        confirmation_prompt: "Receive ALL lines of PO PO00501 into stock? (Full receipt only.)",
        confirmation_token: "CONFIRM_RECEIPT_501_fixture",
      },
    },
  },
  {
    name: "citation — record",
    block: { type: "citation", props: { id: "501", record: "PO00501" } },
  },
  {
    name: "citation — document",
    block: { type: "citation", props: { label: "BILL/2026/0601", url: "https://odoo.example.com/bill/601" } },
  },
  {
    name: "approval",
    block: { type: "approval", props: { confirmation_prompt: "Draft vendor bill from PO PO00501?" } },
  },
];

export default function StorybookPage() {
  const noop = (text: string) => alert(`Fixture action (not sent): ${text}`);
  return (
    <div className="min-h-screen bg-[#f4f4f3] p-8">
      <h1 className="mb-1 text-xl font-semibold tracking-tight text-slate-900">Registry fixtures</h1>
      <p className="mb-6 text-sm text-slate-500">
        Every hand-built GenUI block with representative props. Buttons are inert here.
      </p>
      <div className="grid max-w-3xl grid-cols-1 gap-6">
        {FIXTURES.map((f) => (
          <section key={f.name} className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm">
            <p className="mb-3 font-mono text-[11px] uppercase tracking-wider text-slate-400">{f.name}</p>
            <RenderBlock block={f.block} onAction={noop} />
          </section>
        ))}
      </div>
    </div>
  );
}
