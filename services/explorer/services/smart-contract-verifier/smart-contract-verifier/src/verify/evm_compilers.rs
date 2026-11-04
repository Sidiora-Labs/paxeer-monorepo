use super::{compiler_output::SharedCompilerOutput, compiler_process, Error};
use crate::{
    compiler::DownloadCache, metrics, metrics::GuardedGauge, DetailedVersion, Fetcher, Language,
};
use anyhow::Context;
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::{
    collections::BTreeMap,
    marker::PhantomData,
    path::{Path, PathBuf},
    sync::Arc,
    time::Duration,
};
use tokio::{
    sync::{oneshot, Semaphore},
    time::timeout,
};
use tracing::instrument;

#[async_trait]
pub trait EvmCompiler: Send + Sync + 'static {
    type CompilerInput: CompilerInput;
    type CompilationError: CompilationError + for<'de> Deserialize<'de>;
    // TODO: parameterize version via: `type Version: Version`

    async fn compile(
        compiler_path: &Path,
        compiler_version: &DetailedVersion,
        input: &Self::CompilerInput,
    ) -> Result<Value, Error>;
}

pub trait CompilerInput: Serialize + Clone + Send + Sync + 'static {
    fn normalize_output_selection(&mut self, version: &semver::Version);

    /// Modifies input so that the corresponding bytecode
    /// should have modified metadata hash, if any.
    fn modified_copy(&self) -> Self;

    fn language(&self) -> Language;

    fn settings(&self) -> Value;

    fn sources(&self) -> BTreeMap<String, String>;
}

pub trait CompilationError:
    foundry_compilers::CompilationError + for<'de> Deserialize<'de>
{
    fn formatted_message(&self) -> String;
}

#[derive(Clone, Debug)]
pub struct CompileResult<CompilerOutput> {
    pub output: CompilerOutput,
    pub raw: Value,
}

pub struct EvmCompilersPool<C: EvmCompiler> {
    cache: DownloadCache<DetailedVersion>,
    fetcher: Arc<dyn Fetcher<Version = DetailedVersion>>,
    threads_semaphore: Arc<Semaphore>,
    admission_semaphore: Arc<Semaphore>,
    queue_timeout: Duration,
    execution_timeout: Duration,
    _phantom_data: PhantomData<C>,
}

impl<C: EvmCompiler> EvmCompilersPool<C> {
    pub fn new(
        fetcher: Arc<dyn Fetcher<Version = DetailedVersion>>,
        threads_semaphore: Arc<Semaphore>,
    ) -> Self {
        Self::new_with_limits(
            fetcher,
            threads_semaphore,
            64,
            Duration::from_secs(30),
            compiler_process::DEFAULT_EXECUTION_TIMEOUT,
        )
        .expect("default compiler limits are valid")
    }

    pub fn new_with_limits(
        fetcher: Arc<dyn Fetcher<Version = DetailedVersion>>,
        threads_semaphore: Arc<Semaphore>,
        max_admitted: usize,
        queue_timeout: Duration,
        execution_timeout: Duration,
    ) -> Result<Self, Error> {
        if max_admitted == 0
            || max_admitted > Semaphore::MAX_PERMITS
            || queue_timeout.is_zero()
            || execution_timeout.is_zero()
        {
            return Err(Error::Compilation(vec!["compiler admission limit and deadlines must be positive and within semaphore capacity".into()]));
        }
        Ok(Self {
            cache: Default::default(),
            fetcher,
            threads_semaphore,
            admission_semaphore: Arc::new(Semaphore::new(max_admitted)),
            queue_timeout,
            execution_timeout,
            _phantom_data: Default::default(),
        })
    }

    pub async fn load_from_dir(&self, dir: &PathBuf) {
        match self.cache.load_from_dir(dir).await {
            Ok(_) => {}
            Err(e) => {
                tracing::warn!(
                    "cannot load local compilers from `{}` dir: {}",
                    dir.to_string_lossy(),
                    e
                )
            }
        }
    }

    pub fn normalize_compiler_version(
        &self,
        to_normalize: &DetailedVersion,
    ) -> Result<DetailedVersion, Error> {
        let is_matching_version = |version: &&DetailedVersion| {
            version.version() == to_normalize.version()
                && version.date() == to_normalize.date()
                && (version.commit().starts_with(to_normalize.commit())
                    || to_normalize.commit().starts_with(version.commit()))
        };

        let all_versions = self.fetcher.all_versions();
        let normalized = all_versions.iter().find(is_matching_version);

        if let Some(normalized) = normalized {
            Ok(normalized.clone())
        } else {
            Err(Error::CompilerNotFound(to_normalize.to_string()))
        }
    }

    #[instrument(name = "fetch_compiler", skip(self), level = "debug")]
    pub async fn fetch_compiler(&self, version: &DetailedVersion) -> Result<PathBuf, Error> {
        let path = self
            .cache
            .get(self.fetcher.as_ref(), version)
            .await
            .map_err(|err| {
                Error::Internal(anyhow::Error::new(err).context("fetching evm compiler"))
            })?;
        Ok(path)
    }

    #[instrument(name = "compile", skip(self, input), level = "debug")]
    pub async fn compile(
        &self,
        compiler_path: &Path,
        compiler_version: &DetailedVersion,
        input: &C::CompilerInput,
    ) -> Result<CompileResult<SharedCompilerOutput>, Error> {
        let admission = self
            .admission_semaphore
            .clone()
            .try_acquire_owned()
            .map_err(|_| {
                Error::Compilation(vec!["compiler admission capacity exhausted".into()])
            })?;
        let semaphore = self.threads_semaphore.clone();
        let queue_timeout = self.queue_timeout;
        let execution_timeout = self.execution_timeout;
        let compiler_path = compiler_path.to_path_buf();
        let compiler_version = compiler_version.clone();
        let input = input.clone();
        let (cancel_on_drop, mut cancelled) = oneshot::channel::<()>();
        let compilation = tokio::spawn(async move {
            let _admission = admission;
            let _permit = {
                let _wait_timer_guard = metrics::COMPILATION_QUEUE_TIME.start_timer();
                let _wait_gauge_guard = metrics::COMPILATIONS_IN_QUEUE.guarded_inc();
                tokio::select! {
                    biased;
                    _ = &mut cancelled => return Err(Error::Compilation(vec!["compiler queue cancelled".into()])),
                    permit = timeout(queue_timeout, semaphore.acquire_owned()) => {
                        permit.map_err(|_| Error::Compilation(vec!["compiler queue deadline exceeded".into()]))?
                            .context("acquiring compiler permit")?
                    }
                }
            };
            let _compile_timer_guard = metrics::COMPILE_TIME.start_timer();
            let _compile_gauge_guard = metrics::COMPILATIONS_IN_FLIGHT.guarded_inc();
            compiler_process::supervise(
                execution_timeout,
                &mut cancelled,
                C::compile(&compiler_path, &compiler_version, &input),
            )
            .await
        });
        let raw = compilation.await.context("joining compiler supervisor")?;
        drop(cancel_on_drop);
        let raw = raw?;

        validate_no_errors::<C::CompilationError>(&raw)?;
        let output: SharedCompilerOutput =
            serde_path_to_error::deserialize(&raw).context("deserializing compiler output")?;

        Ok(CompileResult { output, raw })
    }

    pub fn all_versions(&self) -> Vec<DetailedVersion> {
        self.fetcher.all_versions()
    }
}

#[derive(Debug, Clone, Deserialize)]
struct CompilerOutputErrors<E> {
    #[serde(default = "Vec::new")]
    pub errors: Vec<E>,
}

fn validate_no_errors<E: CompilationError>(raw_output: &Value) -> Result<(), Error> {
    let output_errors: CompilerOutputErrors<E> = serde_path_to_error::deserialize(raw_output)
        .context("deserializing compiler output errors")?;

    let mut errors = Vec::new();
    for error in output_errors.errors {
        if error.is_error() {
            errors.push(error.formatted_message());
        }
    }
    if !errors.is_empty() {
        return Err(Error::Compilation(errors));
    }

    Ok(())
}
