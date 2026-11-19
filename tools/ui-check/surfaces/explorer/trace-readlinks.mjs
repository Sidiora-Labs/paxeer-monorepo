export function memoizeReadlink(readlink) {
  const links = new Map();
  return (path) => {
    if (!links.has(path)) links.set(path, readlink(path));
    return links.get(path);
  };
}

export function cacheTraceReadlinks(tracer) {
  const nodeFileTrace = (files, options = {}) => tracer.nodeFileTrace(files, {
    ...options,
    ...(options.readlink ? { readlink: memoizeReadlink(options.readlink) } : {}),
  });
  return new Proxy(tracer, {
    get(target, property, receiver) {
      return property === 'nodeFileTrace' ? nodeFileTrace : Reflect.get(target, property, receiver);
    },
  });
}
