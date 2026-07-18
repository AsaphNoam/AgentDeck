import { Component, type ErrorInfo, type ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  label?: string;
}

interface State {
  error: Error | null;
}

// ErrorBoundary isolates render failures so one bad subtree (e.g. a transcript
// renderer fed malformed server data) does not unmount the whole app.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("ErrorBoundary caught", this.props.label ?? "", error, info.componentStack);
  }

  reset = () => this.setState({ error: null });

  render() {
    if (this.state.error) {
      if (this.props.fallback) return this.props.fallback;
      return (
        <div className="error-boundary" data-ui="error-state" data-state="error" role="alert">
          <p data-slot="message">Something went wrong{this.props.label ? ` in ${this.props.label}` : ""}.</p>
          <div data-slot="actions"><button type="button" onClick={this.reset}>Try again</button></div>
        </div>
      );
    }
    return this.props.children;
  }
}
