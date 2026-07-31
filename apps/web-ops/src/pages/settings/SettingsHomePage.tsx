import { Navigate } from "react-router-dom";

/** Settings hub redirects into the first section — nav lives in the settings sidebar. */
export function SettingsHomePage() {
  return <Navigate to="/settings/staff" replace />;
}
