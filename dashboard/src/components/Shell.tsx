import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../lib/auth";

const nav = [
  { to: "/outreach", label: "Outreach" },
  { to: "/inbox", label: "Inbox" },
  { to: "/analytics", label: "Analytics" },
  { to: "/contacts", label: "Contacts" },
  { to: "/resumes", label: "Resumes" },
  { to: "/settings", label: "Settings" },
];

export function Shell() {
  const { user, signOut } = useAuth();

  return (
    <div className="flex h-full">
      <aside className="flex w-56 flex-col border-r border-gray-200 bg-white p-4">
        <div className="mb-6 px-2 text-lg font-semibold">OneApply</div>
        <nav className="space-y-1">
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
        </nav>
        <div className="mt-auto border-t border-gray-100 pt-4">
          <div className="px-2 text-xs text-gray-500 truncate" title={user?.email}>
            {user?.email}
          </div>
          <button
            onClick={() => signOut()}
            className="mt-2 w-full rounded px-2 py-1 text-left text-xs text-gray-500 hover:bg-gray-100"
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto p-8">
        <Outlet />
      </main>
    </div>
  );
}
