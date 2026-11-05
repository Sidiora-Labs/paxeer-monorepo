import React from "react";

// The declared tsx runner preserves classic JSX. Supply its real React runtime
// explicitly for full-page SSR, including widgets that rely on Next's automatic
// runtime. No component or hook is replaced. The source import wiring is tracked
// in repair-22-33-253.kvx because those widget paths are outside this task.
Object.assign(globalThis, { React });
