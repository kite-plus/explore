import { Component, type ReactNode } from 'react'

type ErrorBoundaryProps = { fallback: ReactNode; children: ReactNode }

/** Stands in for the errorComponent shadcn-admin sets on its routes. */
export class ErrorBoundary extends Component<ErrorBoundaryProps, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children
  }
}
