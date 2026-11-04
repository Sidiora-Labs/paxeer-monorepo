use super::Error;
use anyhow::Context;
use std::{
    future::Future,
    process::{Output, Stdio},
    sync::Arc,
    time::Duration,
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    process::{Child, Command},
    sync::{oneshot, Mutex},
    time::timeout,
};

pub(crate) const DEFAULT_EXECUTION_TIMEOUT: Duration = Duration::from_secs(120);

tokio::task_local! {
    static ACTIVE_PROCESS: Arc<CompilerProcess>;
}

#[derive(Default)]
struct CompilerProcess {
    child: Mutex<Option<Child>>,
}

impl CompilerProcess {
    async fn run(&self, mut command: Command, input: Option<Vec<u8>>) -> Result<Output, Error> {
        let mut slot = self.child.lock().await;
        command
            .kill_on_drop(true)
            .stdin(if input.is_some() {
                Stdio::piped()
            } else {
                Stdio::null()
            })
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        *slot = Some(command.spawn().context("spawning compiler process")?);
        let child = slot.as_mut().expect("compiler process was just spawned");
        let mut stdin = child.stdin.take();
        let mut stdout = child.stdout.take().expect("compiler stdout is piped");
        let mut stderr = child.stderr.take().expect("compiler stderr is piped");

        let write_input = async move {
            if let (Some(mut stdin), Some(input)) = (stdin.take(), input) {
                stdin.write_all(&input).await?;
                stdin.shutdown().await?;
            }
            Ok::<_, std::io::Error>(())
        };
        let read_stdout = async move {
            let mut output = Vec::new();
            stdout.read_to_end(&mut output).await?;
            Ok::<_, std::io::Error>(output)
        };
        let read_stderr = async move {
            let mut output = Vec::new();
            stderr.read_to_end(&mut output).await?;
            Ok::<_, std::io::Error>(output)
        };
        let (status, stdout, stderr, ()) =
            tokio::try_join!(child.wait(), read_stdout, read_stderr, write_input)
                .context("communicating with compiler process")?;
        *slot = None;
        Ok(Output {
            status,
            stdout,
            stderr,
        })
    }

    async fn terminate_and_reap(&self) -> Result<(), Error> {
        let mut slot = self.child.lock().await;
        if let Some(child) = slot.as_mut() {
            if child
                .try_wait()
                .context("checking compiler process")?
                .is_none()
            {
                let termination = child.start_kill();
                child.wait().await.context("reaping compiler process")?;
                termination.context("terminating compiler process")?;
            }
        }
        *slot = None;
        Ok(())
    }
}

pub(crate) async fn supervise<T>(
    execution_timeout: Duration,
    cancelled: &mut oneshot::Receiver<()>,
    operation: impl Future<Output = Result<T, Error>>,
) -> Result<T, Error> {
    let process = Arc::new(CompilerProcess::default());
    let result = ACTIVE_PROCESS.scope(process.clone(), async {
        tokio::select! {
            biased;
            _ = cancelled => Err(Error::Compilation(vec!["compiler execution cancelled".into()])),
            result = timeout(execution_timeout, operation) => {
                result.unwrap_or_else(|_| Err(Error::Compilation(vec!["compiler execution deadline exceeded".into()])))
            }
        }
    }).await;
    process.terminate_and_reap().await?;
    result
}

async fn run_managed_command(command: Command, input: Option<Vec<u8>>) -> Result<Output, Error> {
    let process = ACTIVE_PROCESS.with(Arc::clone);
    process.run(command, input).await
}

pub(crate) async fn run_command(command: Command, input: Option<Vec<u8>>) -> Result<Output, Error> {
    if let Ok(process) = ACTIVE_PROCESS.try_with(Arc::clone) {
        return process.run(command, input).await;
    }

    let (cancel_on_drop, mut cancelled) = oneshot::channel::<()>();
    let supervisor = tokio::spawn(async move {
        supervise(
            DEFAULT_EXECUTION_TIMEOUT,
            &mut cancelled,
            run_managed_command(command, input),
        )
        .await
    });
    let result = supervisor
        .await
        .context("joining compiler process supervisor")?;
    drop(cancel_on_drop);
    result
}
