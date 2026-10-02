import { Alert, Button, Stack } from "@mantine/core";
import { IconAlertTriangle } from "@tabler/icons-react";
import * as React from "react";

class ErrorBoundary extends React.Component<
  { view: string; children: React.ReactNode },
  { error: Error | null }
> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: unknown) {
    return {
      error: error instanceof Error ? error : new Error(String(error)),
    };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("Spec editor error", {
      view: this.props.view,
      name: error.name,
      message: error.message,
      stack: error.stack,
      componentStack: info.componentStack,
      path: window.location.pathname,
      userAgent: navigator.userAgent,
    });
  }

  render() {
    if (this.state.error) {
      return (
        <Alert
          color="red"
          variant="light"
          icon={<IconAlertTriangle size={16} />}
          title="Could not display the configuration"
        >
          <Stack gap="sm">
            <div>
              Your changes are preserved. Try again or switch editor mode.
            </div>
            <Button
              size="compact-xs"
              variant="default"
              className="w-fit"
              onClick={() => this.setState({ error: null })}
            >
              Try again
            </Button>
          </Stack>
        </Alert>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
