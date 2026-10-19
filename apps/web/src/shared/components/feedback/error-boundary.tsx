import { Component, type ErrorInfo, type ReactNode } from "react";
import { isChunkLoadError } from "@/shared/lib/chunk-recovery";
import { createErrorId } from "@/shared/lib/error-id";
import { CrashCard } from "./crash-card";

interface State {
  error: unknown;
  errorId: string | null;
}

/**
 * Batas galat render (OP-01): galat saat render anak tidak lagi mengosongkan layar,
 * melainkan menampilkan CrashCard dengan id kesalahan (dicatat ke console).
 */
export class ErrorBoundary extends Component<{ children: ReactNode; fullScreen?: boolean }, State> {
  state: State = { error: null, errorId: null };

  static getDerivedStateFromError(error: unknown): State {
    return { error: error ?? new Error("unknown"), errorId: createErrorId() };
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error(`[${this.state.errorId}] galat render`, error, info.componentStack);
  }

  render() {
    if (this.state.errorId) {
      return <CrashCard errorId={this.state.errorId} stale={isChunkLoadError(this.state.error)} fullScreen={this.props.fullScreen} />;
    }
    return this.props.children;
  }
}
