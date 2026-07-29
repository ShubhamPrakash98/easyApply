import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import type { OutreachListItem } from "@oneapply/api-client";
import { api } from "../lib/api";
import { OutreachDrawer } from "../components/OutreachDrawer";

const col = createColumnHelper<OutreachListItem>();

const statusStyles: Record<string, string> = {
  pending_approval: "bg-yellow-100 text-yellow-800",
  sent: "bg-blue-100 text-blue-800",
  replied: "bg-green-100 text-green-800",
  followed_up: "bg-indigo-100 text-indigo-800",
  no_response: "bg-gray-100 text-gray-700",
  cancelled: "bg-gray-100 text-gray-500 line-through",
};

const columns = [
  col.accessor("created_at", {
    header: "Date",
    cell: (info) => new Date(info.getValue()).toLocaleDateString(),
    size: 100,
  }),
  col.accessor("recruiter_name", {
    header: "Recruiter",
    cell: (info) => (
      <div className="text-sm">
        <div className="font-medium text-gray-900">{info.getValue()}</div>
        {info.row.original.company && (
          <div className="text-xs text-gray-500">{info.row.original.company}</div>
        )}
      </div>
    ),
  }),
  col.accessor("subject", {
    header: "Subject",
    cell: (info) => (
      <div className="text-sm text-gray-700 truncate max-w-md">{info.getValue()}</div>
    ),
  }),
  col.accessor("status", {
    header: "Status",
    cell: (info) => (
      <span
        className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
          statusStyles[info.getValue()] ?? "bg-gray-100 text-gray-700"
        }`}
      >
        {info.getValue().replace(/_/g, " ")}
      </span>
    ),
    size: 130,
  }),
  col.accessor("follow_up_count", {
    header: "Follow-ups",
    cell: (info) => (
      <span className="text-xs text-gray-500">{info.getValue()}</span>
    ),
    size: 90,
  }),
];

export function Outreach() {
  const [drawerId, setDrawerId] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["outreach", "list"],
    queryFn: () =>
      api.get<{ items: OutreachListItem[] }>("/api/outreach").then((r) => r.items),
    refetchInterval: 5000,
  });

  const table = useReactTable({
    data: data ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <section>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-semibold">Outreach</h1>
        <div className="text-xs text-gray-500">
          {data ? `${data.length} row${data.length === 1 ? "" : "s"}` : ""}
        </div>
      </div>

      {isLoading && <div className="text-sm text-gray-500">Loading…</div>}
      {error && (
        <div className="rounded border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {(error as Error).message}
        </div>
      )}

      {data && data.length === 0 && (
        <div className="rounded border border-dashed border-gray-300 bg-white p-8 text-center">
          <p className="text-gray-600">No outreach yet.</p>
          <p className="mt-1 text-xs text-gray-500">
            Open a LinkedIn recruiter profile and click{" "}
            <span className="font-mono">Reach Out (OneApply)</span>.
          </p>
        </div>
      )}

      {data && data.length > 0 && (
        <div className="overflow-hidden rounded border border-gray-200 bg-white shadow-sm">
          <table className="w-full">
            <thead className="bg-gray-50">
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id}>
                  {hg.headers.map((h) => (
                    <th
                      key={h.id}
                      style={{ width: h.getSize() }}
                      className="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                    >
                      {flexRender(h.column.columnDef.header, h.getContext())}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody className="divide-y divide-gray-100">
              {table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  onClick={() => setDrawerId(row.original.id)}
                  className="cursor-pointer hover:bg-gray-50"
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-4 py-3">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {drawerId && (
        <OutreachDrawer id={drawerId} onClose={() => setDrawerId(null)} />
      )}
    </section>
  );
}
