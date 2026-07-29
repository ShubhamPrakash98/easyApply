import { NavLink, Outlet } from "react-router-dom";

const nav = [
  { to: "/outreach", label: "Outreach" },
  { to: "/inbox", label: "Inbox" },
  { to: "/analytics", label: "Analytics" },
  { to: "/contacts", label: "Contacts" },
  { to: "/resumes", label: "Resumes" },
  { to: "/settings", label: "Settings" },
];

export function Shell() {
  return (
    <div className="flex h-full">
      <aside className="w-56 border-r border-gray-200 bg-white p-4 space-y-1">
        <div className="mb-6 px-2 text-lg font-semibold">OneApply</div>
        {nav.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              `block rounded px-3 py-2 text-sm ${
                isActive
                  ? "bg-indigo-50 text-indigo-700 font-medium"
                  : "text-gray-700 hover:bg-gray-100"
              }`
            }
          >
            {item.label}
          </NavLink>
        ))}
      </aside>
      <main className="flex-1 overflow-auto p-8">
        <Outlet />
      </main>
    </div>
  );
}
