import { Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider } from "./lib/auth";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { Shell } from "./components/Shell";
import { Outreach } from "./routes/Outreach";
import { Inbox } from "./routes/Inbox";
import { Analytics } from "./routes/Analytics";
import { Contacts } from "./routes/Contacts";
import { Resumes } from "./routes/Resumes";
import { Settings } from "./routes/Settings";
import { Login } from "./routes/Login";

export function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          element={
            <ProtectedRoute>
              <Shell />
            </ProtectedRoute>
          }
        >
          <Route path="/" element={<Navigate to="/outreach" replace />} />
          <Route path="/outreach" element={<Outreach />} />
          <Route path="/inbox" element={<Inbox />} />
          <Route path="/analytics" element={<Analytics />} />
          <Route path="/contacts" element={<Contacts />} />
          <Route path="/resumes" element={<Resumes />} />
          <Route path="/settings" element={<Settings />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
