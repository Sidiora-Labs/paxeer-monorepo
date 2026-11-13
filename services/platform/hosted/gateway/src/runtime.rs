//! Embeddable gateway listener using the same configuration and HTTP routes as the binary.

#[allow(dead_code)]
mod application {
    include!("main.rs");

    pub(super) struct Application(Arc<Config>);

    impl Application {
        pub(super) fn configured() -> Result<Self, String> {
            config(false).map(|config| Self(Arc::new(config)))
        }

        pub(super) fn state_clock(mut self, clock: fn() -> Result<u64, String>) -> Self {
            Arc::get_mut(&mut self.0).unwrap_or_else(|| panic!("configuration has not been shared")).state_clock = clock;
            self
        }

        pub(super) fn serve(&self, tcp: TcpStream) -> Result<(), String> {
            serve(&self.0, tcp)
        }
    }
}

pub struct Server(application::Application);

impl Server {
    /// Loads the production gateway configuration, including independent identity credentials.
    ///
    /// # Errors
    ///
    /// Returns an error when required gateway configuration is unavailable or invalid.
    pub fn from_environment() -> Result<Self, String> {
        application::Application::configured().map(Self)
    }

    /// Supplies the trusted wall clock used to check committed state freshness.
    /// The default is the system clock. Recorded transports can use their capture time.
    #[must_use]
    pub fn with_state_clock(self, clock: fn() -> Result<u64, String>) -> Self {
        Self(self.0.state_clock(clock))
    }

    /// Serves an accepted connection through the production parser, authentication and router.
    ///
    /// # Errors
    ///
    /// Returns an error when the connection cannot be served by the gateway.
    pub fn serve_connection(&self, tcp: std::net::TcpStream) -> Result<(), String> {
        self.0.serve(tcp)
    }
}
