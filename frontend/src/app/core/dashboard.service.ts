import { DOCUMENT } from '@angular/common';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { DestroyRef, inject, Injectable, signal } from '@angular/core';
import { Subscription } from 'rxjs';
import { ApiConfiguration } from '../api/api-configuration';
import { getDashboard } from '../api/fn/dashboard/get-dashboard';
import { Dashboard } from '../api/models/dashboard';

/** What the page knows of the dashboard. */
export type DashboardState =
  | { kind: 'loading' }
  /** The last answer; stale when the refresh after it failed. */
  | { kind: 'ready'; dashboard: Dashboard; stale: boolean }
  /** No answer yet: the backend is starting (503 not_ready) or cannot be reached. */
  | { kind: 'failed'; reason: 'starting' | 'unreachable' };

/** How long the page waits before it asks again while it has no answer at all. */
export const retrySeconds = 5;

/**
 * The page's content, GET /api/v1/dashboard: asked on load, every `ui.refresh_seconds`, and at
 * once when the page becomes visible again; never while it is hidden (docs/adr/0001: live data
 * without push — the answer comes from the backend's cache and costs the API server nothing). A
 * refresh that fails keeps the last answer and marks it stale.
 */
@Injectable({ providedIn: 'root' })
export class DashboardService {
  private readonly http = inject(HttpClient);
  private readonly config = inject(ApiConfiguration);
  private readonly document = inject(DOCUMENT);

  readonly state = signal<DashboardState>({ kind: 'loading' });

  private started = false;
  private timer?: ReturnType<typeof setTimeout>;
  private request?: Subscription;
  private readonly onVisibility = () => {
    if (this.document.visibilityState === 'visible') {
      this.refresh();
    } else {
      this.clearTimer();
    }
  };

  constructor() {
    inject(DestroyRef).onDestroy(() => this.stop());
  }

  /** Starts asking; a second call changes nothing. */
  start(): void {
    if (this.started) {
      return;
    }
    this.started = true;
    this.document.addEventListener('visibilitychange', this.onVisibility);
    this.refresh();
  }

  /** Stops asking. */
  stop(): void {
    this.started = false;
    this.document.removeEventListener('visibilitychange', this.onVisibility);
    this.clearTimer();
    this.request?.unsubscribe();
  }

  /** Asks now, and schedules the next question once this one is answered. */
  refresh(): void {
    this.clearTimer();
    this.request?.unsubscribe();
    this.request = getDashboard(this.http, this.config.rootUrl).subscribe({
      next: (response) => {
        this.state.set({ kind: 'ready', dashboard: response.body, stale: false });
        this.schedule(response.body.ui.refresh_seconds);
      },
      error: (error: unknown) => {
        const current = this.state();
        if (current.kind === 'ready') {
          this.state.set({ ...current, stale: true });
          this.schedule(current.dashboard.ui.refresh_seconds);
          return;
        }
        const starting = error instanceof HttpErrorResponse && error.status === 503;
        this.state.set({ kind: 'failed', reason: starting ? 'starting' : 'unreachable' });
        this.schedule(retrySeconds);
      },
    });
  }

  private schedule(seconds: number): void {
    this.clearTimer();
    if (!this.started || this.document.visibilityState === 'hidden') {
      return;
    }
    this.timer = setTimeout(() => this.refresh(), seconds * 1000);
  }

  private clearTimer(): void {
    if (this.timer !== undefined) {
      clearTimeout(this.timer);
      this.timer = undefined;
    }
  }
}
