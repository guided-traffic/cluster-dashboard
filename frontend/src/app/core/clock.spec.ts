import { TestBed } from '@angular/core/testing';
import { Clock, greeting, longDate, timeOfDay } from './clock';

const at = (hour: number) => new Date(2026, 9, 26, hour, 30);

describe('the clock', () => {
  it('names the time of day as Hajimari does', () => {
    expect([4, 5, 11, 12, 17, 18, 21, 22, 0].map((h) => timeOfDay(at(h)))).toEqual([
      'night',
      'morning',
      'morning',
      'afternoon',
      'afternoon',
      'evening',
      'evening',
      'night',
      'night',
    ]);
  });

  it('greets by name, or without one', () => {
    expect(greeting(at(19), 'Hans')).toBe('Good evening, Hans');
    expect(greeting(at(9), '  ')).toBe('Good morning');
  });

  it('writes the date in the given language', () => {
    expect(longDate(at(9), 'en-GB')).toBe('Monday 26 October');
    expect(longDate(at(9), 'de-DE')).toBe('Montag, 26. Oktober');
  });

  it('moves on every minute', () => {
    vi.useFakeTimers();
    vi.setSystemTime(at(11));
    const clock = TestBed.inject(Clock);
    expect(clock.now().getHours()).toBe(11);
    vi.setSystemTime(at(12));
    vi.advanceTimersByTime(60_000);
    expect(clock.now().getHours()).toBe(12);
    TestBed.resetTestingModule();
    vi.useRealTimers();
  });
});
