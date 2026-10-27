"use client";

import React, { useState } from "react";
import { LayoutGrid, BarChart2 } from "lucide-react";

interface GenerativeDataTableProps {
  title: string;
  columns: string[];
  data: any[];
  systemSource?: string; // Kept for backwards compatibility but not rendered in this design
}

export function GenerativeDataTable({ title, columns, data }: GenerativeDataTableProps) {
  const [activeTab, setActiveTab] = useState<"table" | "chart">("table");

  if (!data || data.length === 0) {
    return (
      <div className="rounded-lg border border-zinc-200 bg-white p-4 shadow-sm w-full">
        <p className="text-zinc-500 text-sm">No records found.</p>
      </div>
    );
  }

  // Formatting strings like "amount_total" to "amount total"
  const formatHeader = (key: string) => {
    return key.replace(/_/g, " ").toLowerCase();
  };

  return (
    <div className="w-full rounded-lg border border-zinc-200 bg-white shadow-sm overflow-hidden my-3">
      {/* Header section */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-100">
        <h3 className="text-xs font-semibold text-zinc-800">{title}</h3>
        
        <div className="flex items-center rounded-lg bg-zinc-100 p-0.5">
          <button 
            onClick={() => setActiveTab("table")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[11px] font-medium transition-all ${
              activeTab === "table" ? "bg-white text-zinc-900 shadow-sm" : "text-zinc-500 hover:text-zinc-700"
            }`}
          >
            <LayoutGrid size={12} />
            Table
          </button>
          <button 
            onClick={() => setActiveTab("chart")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[11px] font-medium transition-all ${
              activeTab === "chart" ? "bg-white text-zinc-900 shadow-sm" : "text-zinc-500 hover:text-zinc-700"
            }`}
          >
            <BarChart2 size={12} />
            Chart
          </button>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="w-full">
        {activeTab === "table" ? (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-[#fafafa]">
                <tr>
                  {columns.map((col, idx) => (
                    <th key={idx} className="border-b border-zinc-100 px-4 py-3 font-medium text-zinc-500 whitespace-nowrap">
                      {formatHeader(col)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-100">
                {data.map((row, rIdx) => (
                  <tr key={rIdx} className="bg-white hover:bg-zinc-50 transition-colors">
                    {columns.map((col, cIdx) => (
                      <td key={cIdx} className="px-4 py-3 align-top text-zinc-800 whitespace-nowrap tabular-nums">
                        {
                            typeof row[col] === 'number' && (formatHeader(col).includes('balance') || formatHeader(col).includes('price') || formatHeader(col).includes('amount') || formatHeader(col).includes('total'))
                            ? row[col].toString() // Match exact output in pic 3 (e.g. 2500)
                            : row[col] === null || row[col] === undefined ? "" 
                            : String(row[col])
                        }
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="p-8 flex items-center justify-center text-zinc-400 text-xs">
            Chart visualization coming soon
          </div>
        )}
      </div>
    </div>
  );
}
