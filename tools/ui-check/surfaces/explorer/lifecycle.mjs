export function watchSurfaceOwner(ownerPid, onExit) {
  if (!Number.isSafeInteger(ownerPid) || ownerPid <= 1) {
    throw new Error('Explorer server requires an active owner');
  }
  const timer = setInterval(() => {
    try {
      process.kill(ownerPid, 0);
    } catch (error) {
      if (error.code !== 'ESRCH') throw error;
      clearInterval(timer);
      onExit();
    }
  }, 1000);
  timer.unref();
  return () => clearInterval(timer);
}
