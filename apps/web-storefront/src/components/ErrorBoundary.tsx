import { Component, type ReactNode } from "react";

type Props = { children: ReactNode };
type State = { error: Error | null };

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: { componentStack?: string | null }) {
    // eslint-disable-next-line no-console
    console.error("Storefront crashed:", error.message, info.componentStack);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="panel">
          <h1 className="section-title">Something went wrong</h1>
          <p className="muted">The page hit an unexpected error. Reloading usually fixes it.</p>
          <button type="button" className="btn" onClick={() => window.location.reload()}>
            Reload
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
