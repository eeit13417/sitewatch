import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes } from "react-router";
import { NavBar } from "./components/NavBar";
import { NotFound } from "./components/NotFound";
import { AlertsPage } from "./pages/AlertsPage";
import { DeviceDetail } from "./pages/DeviceDetail";
import { SiteDevices } from "./pages/SiteDevices";
import { SitesOverview } from "./pages/SitesOverview";

const queryClient = new QueryClient();

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <div className="flex min-h-full flex-col">
          <NavBar />
          <main className="flex-1">
            <Routes>
              <Route path="/" element={<SitesOverview />} />
              <Route path="/sites/:siteId" element={<SiteDevices />} />
              <Route path="/devices/:deviceId" element={<DeviceDetail />} />
              <Route path="/alerts" element={<AlertsPage />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </main>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
