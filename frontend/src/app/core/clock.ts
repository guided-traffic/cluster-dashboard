import { DestroyRef, inject, Injectable, signal } from '@angular/core';

/** The time of day a greeting names, as Hajimari greets. */
export function timeOfDay(date: Date): 'morning' | 'afternoon' | 'evening' | 'night' {
  const hour = date.getHours();
  if (hour >= 5 && hour < 12) {
    return 'morning';
  }
  if (hour >= 12 && hour < 18) {
    return 'afternoon';
  }
  if (hour >= 18 && hour < 22) {
    return 'evening';
  }
  return 'night';
}

/** The overview's greeting in its parts — the page sets the time of day apart: "Good *evening*, You." */
export function greeting(date: Date, name: string): { time: string; name: string } {
  return { time: timeOfDay(date), name: name.trim() };
}

/** The date in the overview's bar, in the browser's language: "Monday, 26 October". */
export function longDate(date: Date, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { weekday: 'long', day: 'numeric', month: 'long' }).format(
    date,
  );
}

/** The time beside the date, to the minute, in the browser's language: "19:30". */
export function shortTime(date: Date, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' }).format(date);
}

/** The time the page shows, to the minute: the greeting and the date follow it. */
@Injectable({ providedIn: 'root' })
export class Clock {
  readonly now = signal(new Date());

  constructor() {
    const timer = setInterval(() => this.now.set(new Date()), 60_000);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }
}
