import type { NextRouter } from 'next/router';
import { useRouter } from 'next/router';
import React from 'react';

interface Props {
  children: React.ReactNode;
  renderErrorScreen: (error?: Error) => React.ReactNode;
  onError?: (error: Error) => void;
}

interface PropsWithRouter extends Props {
  router: NextRouter;
}

interface State {
  hasError: boolean;
  error?: Error;
  errorAsPath?: string;
}

class ErrorBoundary extends React.PureComponent<PropsWithRouter, State> {
  state: State = {
    hasError: false,
  };

  static getDerivedStateFromProps(props: PropsWithRouter, state: State) {
    if (state.hasError && state.errorAsPath !== undefined) {
      if (props.router.asPath !== state.errorAsPath) {
        return { hasError: false, error: undefined, errorAsPath: undefined };
      }
    }

    return null;
  }

  componentDidCatch(error: Error) {
    this.setState({ hasError: true, error, errorAsPath: this.props.router.asPath });
    this.props.onError?.(error);
  }

  render() {
    if (this.state.hasError) {
      return this.props.renderErrorScreen(this.state.error);
    }

    return this.props.children;
  }
}

const WrappedErrorBoundary = (props: Props) => {
  const router = useRouter();
  return <ErrorBoundary { ...props } router={ router }/>;
};

export default React.memo(WrappedErrorBoundary);
