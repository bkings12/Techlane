import { Navigate } from "react-router-dom";

/** Counter hub moved onto Job POS quick actions — keep route for old bookmarks. */
export function CounterHomePage() {
  return <Navigate to="/repairs/pos" replace />;
}
