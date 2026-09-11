import { createBrowserRouter, Navigate } from "react-router-dom";
import { publicRoutes } from "@/app/routes/public.routes";
import { printRoutes, protectedRoutes } from "@/app/routes/protected.routes";
import { ROUTE_PATHS } from "@/app/routes/route-paths";

export const router = createBrowserRouter([
  { path: ROUTE_PATHS.home, element: <Navigate to={ROUTE_PATHS.dashboard} replace /> },
  ...publicRoutes,
  ...protectedRoutes,
  ...printRoutes,
  { path: "*", element: <Navigate to={ROUTE_PATHS.login} replace /> },
]);
