import { AppProvider } from "@/app/providers/AppProvider";
import { RouterProvider } from "@/app/providers/RouterProvider";

export default function App() {
  return (
    <AppProvider>
      <RouterProvider />
    </AppProvider>
  );
}
