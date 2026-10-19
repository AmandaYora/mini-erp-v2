import { createBrowserRouter, Navigate } from "react-router-dom";
import { publicRoutes } from "@/app/routes/public.routes";
import { printRoutes, protectedRoutes } from "@/app/routes/protected.routes";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { RouteErrorBoundary } from "@/app/routes/RouteErrorBoundary";

// OP-01: rute induk tanpa path berfungsi sebagai errorElement akar (jaring pengaman layar penuh);
// tiap layout punya errorElement sendiri di protected.routes.tsx agar sidebar tetap tampil.
export const router = createBrowserRouter([
  {
    errorElement: <RouteErrorBoundary fullScreen />,
    children: [
      { path: ROUTE_PATHS.home, element: <Navigate to={ROUTE_PATHS.dashboard} replace /> },
      ...publicRoutes,
      ...protectedRoutes,
      ...printRoutes,
    ],
  },
]);
