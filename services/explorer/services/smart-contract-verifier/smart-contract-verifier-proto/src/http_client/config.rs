use reqwest_middleware::Middleware;
use reqwest_retry::{policies::ExponentialBackoff, RetryTransientMiddleware};
use std::{
    str::FromStr,
    sync::Arc,
    time::{Duration, Instant},
};

type BoxError = Box<dyn std::error::Error + Send + Sync>;

#[derive(Clone)]
pub struct Config {
    url: String,
    middleware_stack: Vec<Arc<dyn Middleware>>,
    probe_url: bool,
    request_timeout: Duration,
    connect_timeout: Duration,
}

impl Config {
    pub fn new(url: String) -> Self {
        Self {
            url,
            middleware_stack: vec![],
            probe_url: false,
            request_timeout: Duration::from_secs(120),
            connect_timeout: Duration::from_secs(10),
        }
    }

    pub fn with_retry_middleware(self, max_retries: u32) -> Self {
        let retry_policy = ExponentialBackoff::builder().build_with_max_retries(max_retries);
        let middleware = RetryTransientMiddleware::new_with_policy(retry_policy);
        self.with_middleware(middleware)
    }

    pub fn with_middleware<M: Middleware>(self, middleware: M) -> Self {
        self.with_arc_middleware(Arc::new(middleware))
    }

    pub fn with_arc_middleware<M: Middleware>(mut self, middleware: Arc<M>) -> Self {
        self.middleware_stack.push(middleware);
        self
    }

    pub fn probe_url(mut self, value: bool) -> Self {
        self.probe_url = value;
        self
    }

    pub fn with_request_timeout(mut self, timeout: Duration) -> Self {
        self.request_timeout = timeout;
        self
    }

    pub fn with_connect_timeout(mut self, timeout: Duration) -> Self {
        self.connect_timeout = timeout;
        self
    }
}

#[derive(Clone)]
pub(super) struct ValidatedConfig {
    pub url: url::Url,
    pub middleware_stack: Vec<Arc<dyn Middleware>>,
    pub probe_url: bool,
    pub request_timeout: Duration,
    pub connect_timeout: Duration,
}

impl TryFrom<Config> for ValidatedConfig {
    type Error = BoxError;

    fn try_from(value: Config) -> std::result::Result<Self, Self::Error> {
        let url = url::Url::from_str(&value.url)?;
        for (name, timeout) in [
            ("request_timeout", value.request_timeout),
            ("connect_timeout", value.connect_timeout),
        ] {
            if timeout.is_zero() || Instant::now().checked_add(timeout).is_none() {
                return Err(
                    format!("{name} must be positive and representable as a deadline").into(),
                );
            }
        }

        Ok(Self {
            url,
            middleware_stack: value.middleware_stack,
            probe_url: value.probe_url,
            request_timeout: value.request_timeout,
            connect_timeout: value.connect_timeout,
        })
    }
}
