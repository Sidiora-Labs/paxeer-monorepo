'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import Image from 'next/image';

interface CarouselSlide {
  src: string;
  alt: string;
  href?: string;
}

interface DiscoverCarouselProps {
  slides: CarouselSlide[];
  autoPlayMs?: number;
  onSlidePress?: (slide: CarouselSlide) => void;
}

export function DiscoverCarousel({
  slides,
  autoPlayMs = 4500,
  onSlidePress,
}: DiscoverCarouselProps) {
  const [active, setActive] = useState(0);
  const [focused, setFocused] = useState(false);
  const trackRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();
  const isDragging = useRef(false);
  const dragStart = useRef(0);

  const goTo = useCallback(
    (idx: number) => {
      const clamped = ((idx % slides.length) + slides.length) % slides.length;
      setActive(clamped);
      trackRef.current?.scrollTo({
        left: clamped * trackRef.current.offsetWidth,
        behavior: 'smooth',
      });
    },
    [slides.length]
  );

  // Auto-play
  useEffect(() => {
    if (focused) return;
    timerRef.current = setTimeout(() => goTo(active + 1), autoPlayMs);
    return () => clearTimeout(timerRef.current);
  }, [active, autoPlayMs, goTo, focused]);

  // Sync dots when user scrolls
  const handleScroll = useCallback(() => {
    if (!trackRef.current) return;
    const idx = Math.round(trackRef.current.scrollLeft / trackRef.current.offsetWidth);
    if (idx !== active) setActive(idx);
  }, [active]);

  // Touch drag start
  const handleTouchStart = (e: React.TouchEvent) => {
    dragStart.current = e.touches[0].clientX;
    isDragging.current = true;
    clearTimeout(timerRef.current);
  };

  const handleTouchEnd = (e: React.TouchEvent) => {
    if (!isDragging.current) return;
    isDragging.current = false;
    const diff = dragStart.current - e.changedTouches[0].clientX;
    if (Math.abs(diff) > 40) goTo(active + (diff > 0 ? 1 : -1));
  };

  return (
    <div
      className="col-span-2 flex flex-col gap-2"
      onFocusCapture={() => {
        clearTimeout(timerRef.current);
        setFocused(true);
      }}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false);
      }}
    >
      {/* Track */}
      <div
        ref={trackRef}
        onScroll={handleScroll}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
        className="flex overflow-x-auto no-scrollbar snap-x snap-mandatory scroll-smooth rounded-3xl"
        style={{ scrollSnapType: 'x mandatory' }}
      >
        {slides.map((slide, i) => (
          <button
            type="button"
            key={i}
            onFocus={(event) => {
              setActive(i);
              trackRef.current?.scrollTo({
                left: i * trackRef.current.offsetWidth,
                behavior: 'instant',
              });
              event.currentTarget.scrollIntoView({
                block: 'nearest',
                inline: 'nearest',
                behavior: 'instant',
              });
            }}
            onClick={() => {
              if (!isDragging.current) onSlidePress?.(slide);
            }}
            className="pax-discover-slide shrink-0 w-full snap-start rounded-xl overflow-hidden relative aspect-[16/9] scroll-my-4"
            style={{ scrollSnapAlign: 'start' }}
          >
            <Image
              src={slide.src}
              alt={slide.alt}
              fill
              sizes="100vw"
              className="object-cover select-none"
              draggable={false}
            />
          </button>
        ))}
      </div>

      {/* Dots */}
      {slides.length > 1 && (
        <div className="flex items-center justify-center gap-2">
          {slides.map((_, i) => (
            <button
              type="button"
              key={i}
              aria-label={`Show slide ${i + 1}`}
              onClick={() => goTo(i)}
              className={`press-scale h-1.5 rounded-full transition-all duration-normal ${
                i === active ? 'w-[var(--space-4)] bg-pax-accent' : 'w-1.5 bg-pax-control'
              }`}
            />
          ))}
        </div>
      )}
    </div>
  );
}
