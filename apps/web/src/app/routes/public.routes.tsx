import type { RouteObject } from "react-router-dom";
import { ROUTE_PATHS } from "@/app/routes/route-paths";
import { RequireAuth, RequireBranch } from "@/app/routes/guards";
import {
  ForbiddenPage,
  LoginPage,
  SelectBranchPage,
} from "@/app/routes/route-pages";

export const publicRoutes: RouteObject[] = [
  { path: ROUTE_PATHS.login, element: <LoginPage /> },
  {
    path: ROUTE_PATHS.selectBranch,
    element: (
      <RequireAuth>
        <RequireBranch allowBranchless>
          <SelectBranchPage />
        </RequireBranch>
      </RequireAuth>
    ),
  },
  { path: ROUTE_PATHS.forbidden, element: <ForbiddenPage /> },
];
