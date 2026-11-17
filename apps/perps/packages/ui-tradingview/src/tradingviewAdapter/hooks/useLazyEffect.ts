import {
  DependencyList,
  EffectCallback,
  useCallback,
  useEffect,
  useRef,
} from "react";

const EMPTY_DEPENDENCIES: DependencyList = [];

function debounce(callback: any, time: any) {
  let interval: number | null;
  return () => {
    if (interval) {
      window.clearTimeout(interval);
    }
    interval = window.setTimeout(() => {
      interval = null;
      callback(arguments);
    }, time);
  };
}

export function useLazyEffect(
  effect: EffectCallback,
  deps: DependencyList = EMPTY_DEPENDENCIES,
  wait = 300,
) {
  const cleanUp = useRef<void | (() => void)>();
  const effectRef = useRef<EffectCallback>();
  const updatedEffect = useCallback(effect, deps);
  effectRef.current = updatedEffect;
  const lazyEffect = useCallback(
    debounce(() => {
      cleanUp.current = effectRef.current?.();
    }, wait),
    [],
  );
  useEffect(lazyEffect, deps);
  useEffect(() => {
    return () => {
      cleanUp.current instanceof Function ? cleanUp.current() : undefined;
    };
  }, []);
}
